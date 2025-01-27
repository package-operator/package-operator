package packageimport

import (
	"context"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/crane"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/utils"
)

// RequestManager de-duplicates multiple parallel container image pulls.
// Has a (semi) in-cluster dependency because it uses `FromRegistryInCluster`.
type RequestManager struct {
	registryHostOverrides map[string]string

	pullImage    pullImageFn
	inFlight     map[string][]chan<- response
	inFlightLock sync.Mutex

	serviceAccount types.NamespacedName
	uncachedClient client.Client
}

type response struct {
	RawPackage *packagetypes.RawPackage
	Err        error
}

type pullImageFn func(
	ctx context.Context, uncachedClient client.Client,
	serviceAccount types.NamespacedName,
	ref string, opts ...crane.Option,
) (*packagetypes.RawPackage, error)

// Creates a new request manager instance to de-duplicate parallel container image pulls.
func NewRequestManager(
	registryHostOverrides map[string]string,
	uncachedClient client.Client, serviceAccount types.NamespacedName,
) *RequestManager {
	return &RequestManager{
		registryHostOverrides: registryHostOverrides,
		pullImage:             FromRegistryInCluster,
		inFlight:              make(map[string][]chan<- response),
		serviceAccount:        serviceAccount,
		uncachedClient:        uncachedClient,
	}
}

func (r *RequestManager) Pull(
	ctx context.Context, image string,
) (*packagetypes.RawPackage, error) {
	image, err := r.applyOverride(image)
	if err != nil {
		return nil, err
	}

	res := <-r.handleRequest(ctx, image)

	return res.RawPackage, res.Err
}

func (r *RequestManager) applyOverride(image string) (string, error) {
	for original, override := range r.registryHostOverrides {
		if strings.HasPrefix(image, original) {
			return utils.ImageURLWithOverride(image, override)
		}
	}
	return image, nil
}

// handleRequest first checks if the provided image is already being pulled.
// If it is not, a new go routine is started to pull the image and trigger
// response handling. Then a new receiver is registered to listen for the response.
// These steps must all occur within the same lock scope to prevent dirty reads
// on the in flight pull requests, more specifically, a check if an image pull
// is in flight after a pull attempt has started, but before the first receiver
// is registered.
func (r *RequestManager) handleRequest(ctx context.Context, image string) <-chan response {
	r.inFlightLock.Lock()
	defer r.inFlightLock.Unlock()

	if _, inFlight := r.inFlight[image]; !inFlight {
		go func(ctx context.Context, image string) {
			rawPkg, err := r.pullImage(ctx, r.uncachedClient, r.serviceAccount, image)
			r.handleResponse(image, response{
				RawPackage: rawPkg,
				Err:        err,
			})
		}(ctx, image)
	}

	// buffer size of 1 ensures that response handler
	// is never blocked by a receiver.
	recv := make(chan response, 1)

	r.inFlight[image] = append(r.inFlight[image], recv)

	return recv
}

// handleResponse broadcasts a response to all receivers listening
// for a given image's pull request and then deletes the image's
// entry allowing new requests to trigger a fresh pull. These
// steps must occur within the same lock scope to prevent dirty
// writes, more specifically, the registration of a new receiver
// after broadcast has occurred, but before the image entry is
// deleted.
func (r *RequestManager) handleResponse(image string, res response) {
	r.inFlightLock.Lock()
	defer r.inFlightLock.Unlock()

	for _, recv := range r.inFlight[image] {
		var rawPkg *packagetypes.RawPackage
		if res.RawPackage != nil {
			// DeepCopy to ensure clients can work concurrently on the returned files map.
			rawPkg = res.RawPackage.DeepCopy()
		}
		recv <- response{
			RawPackage: rawPkg,
			Err:        res.Err,
		}
	}

	delete(r.inFlight, image)
}
