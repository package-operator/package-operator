package packageimport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/utils"
)

func getPullSecret(ctx context.Context, uncachedClient client.Client, name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := uncachedClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	return secret, err
}

func createKeychain(ctx context.Context, uncachedClient client.Client) (authn.Keychain, error) {
	var pullSecrets []corev1.Secret

	saName := types.NamespacedName{Namespace: os.Getenv("PKO_NAMESPACE"), Name: os.Getenv("PKO_SERVICE_ACCOUNT")}

	sa := &corev1.ServiceAccount{}
	err := uncachedClient.Get(ctx, saName, sa)
	switch {
	case err == nil:
		pullSecrets = make([]corev1.Secret, 0, len(sa.ImagePullSecrets))
		for _, localObjectRef := range sa.ImagePullSecrets {
			ps, err := getPullSecret(ctx, uncachedClient, localObjectRef.Name, saName.Namespace)
			switch {
			case err == nil:
				pullSecrets = append(pullSecrets, *ps)
			case k8serrors.IsNotFound(err):
				logs.Warn.Printf("secret %s not found; ignoring", localObjectRef.Name)
			default:
				return nil, err
			}
		}
	case k8serrors.IsNotFound(err):
		logs.Warn.Printf("serviceaccount default; ignoring")
	default:
		return nil, err
	}

	keyring := &keyring{map[string][]authn.AuthConfig{}}

	var cfg struct {
		Auths map[string]authn.AuthConfig `json:"auths"`
	}

	for _, secret := range pullSecrets {
		jsonCfg, jsonCfgExists := secret.Data[corev1.DockerConfigJsonKey]
		baseCfg, cfgExists := secret.Data[corev1.DockerConfigKey]
		switch {
		case secret.Type == corev1.SecretTypeDockerConfigJson && jsonCfgExists && len(jsonCfg) > 0:
			if err := json.Unmarshal(jsonCfg, &cfg); err != nil {
				return nil, err
			}
		case secret.Type == corev1.SecretTypeDockercfg && cfgExists && len(baseCfg) > 0:
			if err := json.Unmarshal(baseCfg, &cfg.Auths); err != nil {
				return nil, err
			}
		}

		for registry, v := range cfg.Auths {
			if !strings.HasPrefix(registry, "https://") && !strings.HasPrefix(registry, "http://") {
				registry = "https://" + registry
			}
			parsed, err := url.Parse(registry)
			if err != nil {
				return nil, fmt.Errorf("entry %q in dockercfg invalid (%w)", registry, err)
			}

			effectivePath := parsed.Path
			if strings.HasPrefix(effectivePath, "/v2/") || strings.HasPrefix(effectivePath, "/v1/") {
				effectivePath = effectivePath[3:]
			}
			key := parsed.Host
			if (effectivePath != "") && (effectivePath != "/") {
				key += effectivePath
			}

			keyring.creds[key] = append(keyring.creds[key], v)
		}
	}
	return keyring, nil
}

type keyring struct {
	creds map[string][]authn.AuthConfig
}

func (keyring *keyring) Resolve(target authn.Resource) (authn.Authenticator, error) {
	image := target.String()
	auths := []authn.AuthConfig{}

	for idx, creds := range keyring.creds {
		if idx == image {
			auths = append(auths, creds...)
		}
	}

	if len(auths) == 0 {
		return authn.Anonymous, nil
	}

	auth := auths[0]
	auth.Auth = ""
	return authn.FromConfig(auth), nil
}

// Imports a RawPackage from a container image registry.
func FromRegistry(
	ctx context.Context, uncachedClient client.Client, ref string, opts ...crane.Option,
) (*packagetypes.RawPackage, error) {
	chain, err := createKeychain(ctx, uncachedClient)
	if err != nil {
		return nil, err
	}
	img, err := crane.Pull(ref, append(opts, crane.WithAuthFromKeychain(chain))...)
	if err != nil {
		return nil, err
	}
	return FromOCI(ctx, img)
}

// Registry de-duplicates multiple parallel container image pulls.
type Registry struct {
	registryHostOverrides map[string]string

	pullImage    pullImageFn
	inFlight     map[string][]chan<- response
	inFlightLock sync.Mutex
}

type response struct {
	RawPackage *packagetypes.RawPackage
	Err        error
}

type pullImageFn func(
	ctx context.Context, uncachedClient client.Client, ref string, opts ...crane.Option) (*packagetypes.RawPackage, error)

// Creates a new registry instance to de-duplicate parallel container image pulls.
func NewRegistry(registryHostOverrides map[string]string) *Registry {
	return &Registry{
		registryHostOverrides: registryHostOverrides,
		pullImage:             FromRegistry,
		inFlight:              make(map[string][]chan<- response),
	}
}

func (r *Registry) Pull(
	ctx context.Context, uncachedClient client.Client, image string,
) (*packagetypes.RawPackage, error) {
	image, err := r.applyOverride(image)
	if err != nil {
		return nil, err
	}

	res := <-r.handleRequest(ctx, uncachedClient, image)

	return res.RawPackage, res.Err
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
func (r *Registry) handleRequest(ctx context.Context, uncachedClient client.Client, image string) <-chan response {
	r.inFlightLock.Lock()
	defer r.inFlightLock.Unlock()

	if _, inFlight := r.inFlight[image]; !inFlight {
		go func(ctx context.Context, image string) {
			rawPkg, err := r.pullImage(ctx, uncachedClient, image)
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
func (r *Registry) handleResponse(image string, res response) {
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
