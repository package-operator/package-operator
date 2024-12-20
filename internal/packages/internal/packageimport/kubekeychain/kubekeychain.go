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

package kubekeychain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func getPullSecret(ctx context.Context, uncachedClient client.Client, name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := uncachedClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	return secret, err
}

func New(ctx context.Context, uncachedClient client.Client) (authn.Keychain, error) {
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
