package packageimport

import (
	"context"
	"strings"
	"sync"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/utils"
)

// Registry handles pulling images from a registry during PKO runtime.
type Registry struct {
	registryHostOverrides map[string]string

	pullImage    pullImageFn
	inFlight     map[inFlightKey][]chan<- response
	inFlightLock sync.Mutex
}

type inFlightKey struct {
	image   string
	pkgType corev1alpha1.PackageType
}

type pullImageFn func(ctx context.Context, ref string, pkgType corev1alpha1.PackageType) (packagecontent.Files, error)

func NewRegistry(registryHostOverrides map[string]string) *Registry {
	return &Registry{
		registryHostOverrides: registryHostOverrides,
		pullImage:             PulledImage,
		inFlight:              make(map[inFlightKey][]chan<- response),
	}
}

func (r *Registry) Pull(
	ctx context.Context, image string,
	pkgType corev1alpha1.PackageType,
) (packagecontent.Files, error) {
	image, err := r.applyOverride(image)
	if err != nil {
		return nil, err
	}

	res := <-r.handleRequest(ctx, image, pkgType)

	return res.Files, res.Err
}

func (r *Registry) applyOverride(image string) (string, error) {
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
func (r *Registry) handleRequest(
	ctx context.Context, image string,
	pkgType corev1alpha1.PackageType,
) <-chan response {
	r.inFlightLock.Lock()
	defer r.inFlightLock.Unlock()

	key := inFlightKey{
		image:   image,
		pkgType: pkgType,
	}

	if _, inFlight := r.inFlight[key]; !inFlight {
		go func(ctx context.Context, image string, pkgType corev1alpha1.PackageType) {
			files, err := r.pullImage(ctx, image, pkgType)

			r.handleResponse(key, response{
				Files: files,
				Err:   err,
			})
		}(ctx, image, pkgType)
	}

	// buffer size of 1 ensures that response handler
	// is never blocked by a receiver.
	recv := make(chan response, 1)

	r.inFlight[key] = append(r.inFlight[key], recv)

	return recv
}

// handleResponse broadcasts a response to all receivers listening
// for a given image's pull request and then deletes the image's
// entry allowing new requests to trigger a fresh pull. These
// steps must occur within the same lock scope to prevent dirty
// writes, more specifically, the registration of a new receiver
// after broadcast has occurred, but before the image entry is
// deleted.
func (r *Registry) handleResponse(key inFlightKey, res response) {
	r.inFlightLock.Lock()
	defer r.inFlightLock.Unlock()

	for _, recv := range r.inFlight[key] {
		recv <- response{
			// DeepCopy to ensure clients can work concurrently on the returned files map.
			Files: res.Files.DeepCopy(),
			Err:   res.Err,
		}
	}

	delete(r.inFlight, key)
}

type response struct {
	Files packagecontent.Files
	Err   error
}
