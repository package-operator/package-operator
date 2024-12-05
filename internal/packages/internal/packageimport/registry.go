package packageimport

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
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

const (
	// NoServiceAccount is a constant that can be passed via ServiceAccountName
	// to tell the keychain that looking up the service account is unnecessary.
	// This value cannot collide with an actual service account name because
	// service accounts do not allow spaces.
	NoServiceAccount = "no service account"
)

// Options holds configuration data for guiding credential resolution.
type Options struct {
	// Namespace holds the namespace inside of which we are resolving service
	// account and pull secret references to access the image.
	// If empty, "default" is assumed.
	Namespace string

	// ServiceAccountName holds the serviceaccount (within Namespace) as which a
	// Pod might access the image.  Service accounts may have image pull secrets
	// attached, so we lookup the service account to complete the keychain.
	// If empty, "default" is assumed.  To avoid a service account lookup, pass
	// NoServiceAccount explicitly.
	ServiceAccountName string

	// ImagePullSecrets holds the names of the Kubernetes secrets (scoped to
	// Namespace) containing credential data to use for the image pull.
	ImagePullSecrets []string
}

func getPullSecret(ctx context.Context, uncachedClient client.Client, name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := uncachedClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	return secret, err
}

// New returns a new authn.Keychain suitable for resolving image references as
// scoped by the provided Options.  It speaks to Kubernetes through the provided
// client interface.
func New(ctx context.Context, uncachedClient client.Client, opt Options) (authn.Keychain, error) {
	if opt.Namespace == "" {
		opt.Namespace = "default"
	}
	if opt.ServiceAccountName == "" {
		opt.ServiceAccountName = "default"
	}

	// Implement a Kubernetes-style authentication keychain.
	// This needs to support roughly the following kinds of authentication:
	//  1) The explicit authentication from imagePullSecrets on Pod
	//  2) The semi-implicit authentication where imagePullSecrets are on the
	//    Pod's service account.

	// First, fetch all of the explicitly declared pull secrets
	pullSecrets := make([]corev1.Secret, 0, len(opt.ImagePullSecrets))
	for _, name := range opt.ImagePullSecrets {
		ps, err := getPullSecret(ctx, uncachedClient, name, opt.Namespace)
		switch {
		case err == nil:
			pullSecrets = append(pullSecrets, *ps)
		case k8serrors.IsNotFound(err):
			logs.Warn.Printf("secret %s/%s not found; ignoring", opt.Namespace, name)
		default:
			return nil, err
		}
	}
	// TODO
	// Second, fetch all of the pull secrets attached to our service account,
	// unless the user has explicitly specified that no service account lookup
	// is desired.
	if opt.ServiceAccountName != NoServiceAccount {
		sa := &corev1.ServiceAccount{}
		err := uncachedClient.Get(ctx, types.NamespacedName{Namespace: opt.Namespace, Name: opt.ServiceAccountName}, sa)
		switch {
		case err == nil:
			for _, localObjectRef := range sa.ImagePullSecrets {
				ps, err := getPullSecret(ctx, uncachedClient, localObjectRef.Name, opt.Namespace)
				switch {
				case err == nil:
					pullSecrets = append(pullSecrets, *ps)
				case k8serrors.IsNotFound(err):
					logs.Warn.Printf("secret %s/%s not found; ignoring", opt.Namespace, localObjectRef.Name)
				default:
					return nil, err
				}
			}
		case k8serrors.IsNotFound(err):
			logs.Warn.Printf("serviceaccount %s/%s not found; ignoring", opt.Namespace, opt.ServiceAccountName)
		default:
			return nil, err
		}
	}

	keyring := &keyring{
		creds: map[string][]authn.AuthConfig{},
	}

	var cfg dockerConfigJSON

	//nolint: lll
	// From: https://github.com/kubernetes/kubernetes/blob/0dcafb1f37ee522be3c045753623138e5b907001/pkg/credentialprovider/keyring.go
	for _, secret := range pullSecrets {
		if b, exists := secret.Data[corev1.DockerConfigJsonKey]; secret.Type == corev1.SecretTypeDockerConfigJson && exists && len(b) > 0 {
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, err
			}
		}
		if b, exists := secret.Data[corev1.DockerConfigKey]; secret.Type == corev1.SecretTypeDockercfg && exists && len(b) > 0 {
			if err := json.Unmarshal(b, &cfg.Auths); err != nil {
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

			// The docker client allows exact matches:
			//    foo.bar.com/namespace
			// Or hostname matches:
			//    foo.bar.com
			// It also considers /v2/  and /v1/ equivalent to the hostname
			// See ResolveAuthConfig in docker/registry/auth.go.
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

type dockerConfigJSON struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

type keyring struct {
	creds map[string][]authn.AuthConfig
}

func (keyring *keyring) Resolve(target authn.Resource) (authn.Authenticator, error) {
	image := target.String()
	auths := []authn.AuthConfig{}

	for idx, creds := range keyring.creds {
		// both k and image are schemeless URLs because even though schemes are allowed
		// in the credential configurations, we remove them when constructing the keyring
		if matched, _ := urlsMatchStr(idx, image); matched {
			auths = append(auths, creds...)
		}
	}

	if len(auths) == 0 {
		return authn.Anonymous, nil
	}

	return toAuthenticator(auths)
}

// urlsMatchStr is wrapper for URLsMatch, operating on strings instead of URLs.
func urlsMatchStr(glob string, target string) (bool, error) {
	globURL, err := parseSchemelessURL(glob)
	if err != nil {
		return false, err
	}
	targetURL, err := parseSchemelessURL(target)
	if err != nil {
		return false, err
	}
	return urlsMatch(globURL, targetURL)
}

// parseSchemelessURL parses a schemeless url and returns a url.URL
// url.Parse require a scheme, but ours don't have schemes.  Adding a
// scheme to make url.Parse happy, then clear out the resulting scheme.
func parseSchemelessURL(schemelessURL string) (*url.URL, error) {
	parsed, err := url.Parse("https://" + schemelessURL)
	if err != nil {
		return nil, err
	}
	// clear out the resulting scheme
	parsed.Scheme = ""
	return parsed, nil
}

// splitURL splits the host name into parts, as well as the port.
func splitURL(url *url.URL) (parts []string, port string) {
	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		// could not parse port
		host, port = url.Host, ""
	}
	return strings.Split(host, "."), port
}

// urlsMatch checks whether the given target url matches the glob url, which may have
// glob wild cards in the host name.
//
// Examples:
//
//	globURL=*.docker.io, targetURL=blah.docker.io => match
//	globURL=*.docker.io, targetURL=not.right.io   => no match
//
// Note that we don't support wildcards in ports and paths yet.
func urlsMatch(globURL *url.URL, targetURL *url.URL) (bool, error) {
	globURLParts, globPort := splitURL(globURL)
	targetURLParts, targetPort := splitURL(targetURL)
	if globPort != targetPort {
		// port doesn't match
		return false, nil
	}
	if len(globURLParts) != len(targetURLParts) {
		// host name does not have the same number of parts
		return false, nil
	}
	if !strings.HasPrefix(targetURL.Path, globURL.Path) {
		// the path of the credential must be a prefix
		return false, nil
	}
	for k, globURLPart := range globURLParts {
		targetURLPart := targetURLParts[k]
		matched, err := filepath.Match(globURLPart, targetURLPart)
		if err != nil {
			return false, err
		}
		if !matched {
			// glob mismatch for some part
			return false, nil
		}
	}
	// everything matches
	return true, nil
}

func toAuthenticator(configs []authn.AuthConfig) (authn.Authenticator, error) {
	cfg := configs[0]

	if cfg.Auth != "" {
		cfg.Auth = ""
	}

	return authn.FromConfig(cfg), nil
}

// Imports a RawPackage from a container image registry.
func FromRegistry(
	ctx context.Context, uncachedClient client.Client, ref string, opts ...crane.Option,
) (*packagetypes.RawPackage, error) {
	chain, err := New(ctx, uncachedClient, Options{})
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
