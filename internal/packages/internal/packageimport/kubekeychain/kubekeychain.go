// Copyright 2022 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This code was directly inspired and copied with minimal modifications.
//nolint:lll
// The original source for this file can be found at: https://github.com/google/go-containerregistry/blob/6bce25ecf0297c1aa9072bc665b5cf58d53e1c54/pkg/authn/kubernetes/keychain.go

//nolint:lll
package kubekeychain

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Instanciates an authn.Keychain with all PullSecrets that are attached to the given ServiceAccount.
// The ServiceAccount and PullSecret objects will be fetched immediately.
func FromServiceAccountPullSecrets(
	ctx context.Context, uncachedClient client.Client,
	serviceAccount types.NamespacedName,
) (authn.Keychain, error) {
	secrets, err := resolveSecrets(ctx, serviceAccount, uncachedClient)
	if err != nil {
		return nil, err
	}

	return newFromPullSecrets(secrets)
}

func resolveSecrets(ctx context.Context, serviceAccountName types.NamespacedName, uncachedClient client.Client) ([]*corev1.Secret, error) {
	// Fetch ServiceAccount.
	serviceAccount := &corev1.ServiceAccount{}
	if err := uncachedClient.Get(ctx, serviceAccountName, serviceAccount); err != nil {
		return nil, fmt.Errorf("getting service account: %w", err)
	}

	// Fetch all pull secrets attached to ServiceAccount.
	pullSecrets := make([]*corev1.Secret, 0, len(serviceAccount.ImagePullSecrets))
	for _, localRef := range serviceAccount.ImagePullSecrets {
		secret := &corev1.Secret{}

		if err := uncachedClient.Get(ctx, types.NamespacedName{
			Namespace: serviceAccount.Namespace,
			Name:      localRef.Name,
		}, secret); k8serrors.IsNotFound(err) {
			// TODO ignore? should we?
			// TODO wrong logger package
			logs.Warn.Printf("secret %s not found; ignoring", localRef.Name)
			continue
		} else if err != nil {
			return nil, fmt.Errorf("getting pull secret attached to sa: %w", err)
		}

		pullSecrets = append(pullSecrets, secret)
	}

	return pullSecrets, nil
}

type dockerConfigJSON struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

// newFromPullSecrets returns a new authn.Keychain suitable for resolving image references as
// scoped by the pull secrets.
func newFromPullSecrets(secrets []*corev1.Secret) (authn.Keychain, error) {
	keyring := &keyring{
		index: make([]string, 0),
		creds: make(map[string][]authn.AuthConfig),
	}

	var cfg dockerConfigJSON

	// From: https://github.com/kubernetes/kubernetes/blob/0dcafb1f37ee522be3c045753623138e5b907001/pkg/credentialprovider/keyring.go
	for _, secret := range secrets {
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
			value := registry
			if !strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
				value = "https://" + value
			}
			parsed, err := url.Parse(value)
			if err != nil {
				return nil, fmt.Errorf("entry %q in dockercfg invalid (%w)", value, err)
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
			var key string
			if (len(effectivePath) > 0) && (effectivePath != "/") {
				key = parsed.Host + effectivePath
			} else {
				key = parsed.Host
			}

			if _, ok := keyring.creds[key]; !ok {
				keyring.index = append(keyring.index, key)
			}

			keyring.creds[key] = append(keyring.creds[key], v)
		}

		// We reverse sort in to give more specific (aka longer) keys priority
		// when matching for creds
		sort.Sort(sort.Reverse(sort.StringSlice(keyring.index)))
	}
	return keyring, nil
}

type keyring struct {
	index []string
	creds map[string][]authn.AuthConfig
}

func (keyring *keyring) Resolve(target authn.Resource) (authn.Authenticator, error) {
	image := target.String()
	auths := []authn.AuthConfig{}

	for _, k := range keyring.index {
		// both k and image are schemeless URLs because even though schemes are allowed
		// in the credential configurations, we remove them when constructing the keyring
		if matched, _ := urlsMatchStr(k, image); matched {
			auths = append(auths, keyring.creds[k]...)
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
