package kubekeychain

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/testutil"
)

const namespace = "package-operator-system"

func newExpectedSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func TestResolveSecrets_Success(t *testing.T) {
	t.Parallel()

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "package-operator",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
			{Name: "d"},
		},
	}
	expectedSecrets := []*corev1.Secret{
		newExpectedSecret("a"),
		newExpectedSecret("b"),
		newExpectedSecret("c"),
		newExpectedSecret("d"),
	}

	c := testutil.NewClient()

	c.On("Get", mock.Anything, client.ObjectKey{
		Namespace: serviceAccount.Namespace,
		Name:      serviceAccount.Name,
	}, mock.IsType(&corev1.ServiceAccount{}), mock.Anything).Run(func(args mock.Arguments) {
		sa := args.Get(2).(*corev1.ServiceAccount)
		*sa = *serviceAccount
	}).Return(nil)

	for _, localRef := range serviceAccount.ImagePullSecrets {
		c.On("Get", mock.Anything, client.ObjectKey{
			Namespace: serviceAccount.Namespace,
			Name:      localRef.Name,
		}, mock.IsType(&corev1.Secret{}), mock.Anything).Run(func(args mock.Arguments) {
			key := args.Get(1).(client.ObjectKey)
			index := slices.IndexFunc(expectedSecrets, func(secret *corev1.Secret) bool {
				return key.Name == secret.Name && key.Namespace == secret.Namespace
			})
			require.NotEqual(t, -1, index)
			s := args.Get(2).(*corev1.Secret)
			*s = *(expectedSecrets[index].DeepCopy())
		}).Return(nil)
	}

	actual, err := resolveSecrets(context.Background(), client.ObjectKeyFromObject(serviceAccount), c)
	require.NoError(t, err)
	assert.Equal(t, expectedSecrets, actual)
}

func TestResolveSecrets_GetServiceAccountError(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()

	key := client.ObjectKey{
		Namespace: "package-operator-system",
		Name:      "package-operator",
	}

	c.On("Get", mock.Anything, key, mock.IsType(&corev1.ServiceAccount{}), mock.Anything).
		Return(k8sapierrors.NewNotFound(schema.GroupResource{}, ""))

	_, err := resolveSecrets(context.Background(), key, c)
	require.Error(t, err)
	assert.True(t, k8sapierrors.IsNotFound(err))
}

func TestResolveSecrets_GetSecretError(t *testing.T) {
	t.Parallel()

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "package-operator-system",
			Name:      "package-operator",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
			{Name: "d"},
		},
	}

	c := testutil.NewClient()

	c.On("Get", mock.Anything, client.ObjectKey{
		Namespace: serviceAccount.Namespace,
		Name:      serviceAccount.Name,
	}, mock.IsType(&corev1.ServiceAccount{}), mock.Anything).Run(func(args mock.Arguments) {
		sa := args.Get(2).(*corev1.ServiceAccount)
		*sa = *serviceAccount
	}).Return(nil)

	for _, localRef := range serviceAccount.ImagePullSecrets {
		c.On("Get", mock.Anything, client.ObjectKey{
			Namespace: serviceAccount.Namespace,
			Name:      localRef.Name,
		}, mock.IsType(&corev1.Secret{}), mock.Anything).
			Return(k8sapierrors.NewBadRequest(""))
	}

	_, err := resolveSecrets(context.Background(), client.ObjectKeyFromObject(serviceAccount), c)
	require.Error(t, err)
	assert.True(t, k8sapierrors.IsBadRequest(err))
}

func TestResolveSecrets_GetSecretError_IgnoreNotFound(t *testing.T) {
	t.Parallel()

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "package-operator-system",
			Name:      "package-operator",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
			{Name: "d"},
		},
	}

	c := testutil.NewClient()

	c.On("Get", mock.Anything, client.ObjectKey{
		Namespace: serviceAccount.Namespace,
		Name:      serviceAccount.Name,
	}, mock.IsType(&corev1.ServiceAccount{}), mock.Anything).Run(func(args mock.Arguments) {
		sa := args.Get(2).(*corev1.ServiceAccount)
		*sa = *serviceAccount
	}).Return(nil)

	c.On("Get", mock.Anything, mock.Anything, mock.IsType(&corev1.Secret{}), mock.Anything).
		Return(k8sapierrors.NewNotFound(schema.GroupResource{}, ""))

	secrets, err := resolveSecrets(context.Background(), client.ObjectKeyFromObject(serviceAccount), c)
	require.NoError(t, err)
	assert.Empty(t, secrets)
}

func TestKubernetesAuth(t *testing.T) {
	t.Parallel()

	// From https://github.com/knative/serving/issues/12761#issuecomment-1097441770
	// All of these should work with K8s' docker auth parsing.
	for k, ss := range map[string][]string{
		"registry.gitlab.com/dprotaso/test/nginx": {
			"registry.gitlab.com",
			"http://registry.gitlab.com",
			"https://registry.gitlab.com",
			"registry.gitlab.com/dprotaso",
			"http://registry.gitlab.com/dprotaso",
			"https://registry.gitlab.com/dprotaso",
			"registry.gitlab.com/dprotaso/test",
			"http://registry.gitlab.com/dprotaso/test",
			"https://registry.gitlab.com/dprotaso/test",
			"registry.gitlab.com/dprotaso/test/nginx",
			"http://registry.gitlab.com/dprotaso/test/nginx",
			"https://registry.gitlab.com/dprotaso/test/nginx",
		},
		"dtestcontainer.azurecr.io/dave/nginx": {
			"dtestcontainer.azurecr.io",
			"http://dtestcontainer.azurecr.io",
			"https://dtestcontainer.azurecr.io",
			"dtestcontainer.azurecr.io/dave",
			"http://dtestcontainer.azurecr.io/dave",
			"https://dtestcontainer.azurecr.io/dave",
			"dtestcontainer.azurecr.io/dave/nginx",
			"http://dtestcontainer.azurecr.io/dave/nginx",
			"https://dtestcontainer.azurecr.io/dave/nginx",
		},
	} {
		repo, err := name.NewRepository(k)
		if err != nil {
			t.Errorf("parsing %q: %v", k, err)
			continue
		}

		for _, s := range ss {
			t.Run(fmt.Sprintf("%s - %s", k, s), func(t *testing.T) {
				t.Parallel()

				username, password := "foo", "bar"
				kc, err := newFromPullSecrets([]*corev1.Secret{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "ns",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(
							fmt.Sprintf(`{"auths":{%q:{"username":%q,"password":%q,"auth":%q}}}`,
								s,
								username, password,
								base64.StdEncoding.EncodeToString([]byte(username+":"+password))),
						),
					},
				}})
				if err != nil {
					t.Fatalf("NewFromPullSecrets() = %v", err)
				}
				auth, err := kc.Resolve(repo)
				if err != nil {
					t.Errorf("Resolve(%v) = %v", repo, err)
				}
				got, err := auth.Authorization()
				if err != nil {
					t.Errorf("Authorization() = %v", err)
				}
				want, err := (&authn.Basic{Username: username, Password: password}).Authorization()
				if err != nil {
					t.Errorf("Authorization() = %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Resolve() = %v, want %v", got, want)
				}
			})
		}
	}
}
