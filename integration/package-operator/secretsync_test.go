//go:build integration

package packageoperator

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const secretSyncFieldOwner = "secret-sync-integration-test"

func applyObjectNoCleanup(ctx context.Context, t *testing.T, obj client.Object) {
	t.Helper()
	require.NoError(t, Client.Patch(ctx, obj, client.Apply, client.FieldOwner(secretSyncFieldOwner)))
}

func applyObjectWithCleanup(ctx context.Context, t *testing.T, obj client.Object) {
	t.Helper()

	applyObjectNoCleanup(ctx, t, obj)

	t.Cleanup(func() {
		require.NoError(t, client.IgnoreNotFound(Client.Delete(ctx, obj)))
	})
}

func convertStringToByteMap(data map[string]string) map[string][]byte {
	m := map[string][]byte{}
	for k, v := range data {
		m[k] = []byte(v)
	}
	return m
}

func convertByteToStringMap(data map[string][]byte) map[string]string {
	m := map[string]string{}
	for k, v := range data {
		m[k] = string(v)
	}
	return m
}

func newSourceSecret(key types.NamespacedName, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Data: convertStringToByteMap(data),
	}
}

func createTestNamespaceWithCleanup(ctx context.Context, t *testing.T, name string) {
	t.Helper()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	require.NoError(t, Client.Create(ctx, ns, client.FieldOwner(secretSyncFieldOwner)))

	t.Cleanup(func() {
		require.NoError(t, Client.Delete(ctx, ns))
	})
}

func assertAPISecretData(ctx context.Context, t *testing.T, key types.NamespacedName, expected map[string]string) {
	t.Helper()

	secret := &corev1.Secret{}
	require.NoError(t, Client.Get(ctx, key, secret))
	require.Equal(t, expected, convertByteToStringMap(secret.Data))
}

func assertSecretNotFound(ctx context.Context, t *testing.T, key types.NamespacedName) {
	t.Helper()

	require.True(t, apimachineryerrors.IsNotFound(Client.Get(ctx, key, &corev1.Secret{})))
}

func TestSecretSync_Happy(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	type tcase struct {
		name string
		// Src and Dst namespaces MUST not exist.
		// Dst namespaces are allowed to be non-unique within the list.
		src  types.NamespacedName
		dsts []types.NamespacedName
	}

	tcases := []tcase{
		{
			name: "MultipleTargetsInSingleNamespace",
			src: types.NamespacedName{
				Namespace: "secretsync-src-1",
				Name:      "src-simple",
			},
			dsts: []types.NamespacedName{
				{Namespace: "secretsync-dst-1", Name: "dst-simple-1"},
				{Namespace: "secretsync-dst-1", Name: "dst-simple-1"},
			},
		},
		{
			name: "MultipleTargetsInSourceNamespace",
			src: types.NamespacedName{
				Namespace: "secretsync-src-dest",
				Name:      "src-multiple-targets-in-source-namespace",
			},
			dsts: []types.NamespacedName{
				{Namespace: "secretsync-src-dest", Name: "dst-simple-1"},
				{Namespace: "secretsync-src-dest", Name: "dst-simple-1"},
			},
		},
		{
			name: "MultipleTargetsInSeparateNamespaces",
			src: types.NamespacedName{
				Namespace: "secretsync-src-dest",
				Name:      "src-multiple-targets-in-source-namespace",
			},
			dsts: []types.NamespacedName{
				{Namespace: "secretsync-dest-1", Name: "dst-simple-1"},
				{Namespace: "secretsync-dest-2", Name: "dst-simple-2"},
				{Namespace: "secretsync-dest-3", Name: "dst-simple-1"},
			},
		},
		{
			name: "MultipleTargetsInMixedNamespaces",
			src: types.NamespacedName{
				Namespace: "secretsync-src-dest",
				Name:      "src-multiple-targets-in-source-namespace",
			},
			dsts: []types.NamespacedName{
				{Namespace: "secretsync-dest-1", Name: "dst-simple-1"},
				{Namespace: "secretsync-dest-1", Name: "dst-simple-2"},
				{Namespace: "secretsync-dest-2", Name: "dst-simple-3"},
			},
		},
	}

	// dataSteps MUST have at least len(2).
	dataSteps := []map[string]string{
		{"foo": "bar"},
		{"foo": "two", "banana": "dance"},
		{"foo": "three", "banana": "dance", "socken": "affe"},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			// Assert proper test data.
			require.GreaterOrEqual(t, len(dataSteps), 2)
			require.GreaterOrEqual(t, len(tcase.dsts), 1)

			// Lookup table to de-duplicate namespace creation requests.
			known := map[string]struct{}{}

			// Create source namespace.
			createTestNamespaceWithCleanup(ctx, t, tcase.src.Namespace)
			known[tcase.src.Namespace] = struct{}{}

			// Create all unique destination namespaces.
			for _, dst := range tcase.dsts {
				// Skip namespaces that were aleady created.
				if _, ok := known[dst.Namespace]; ok {
					continue
				}
				createTestNamespaceWithCleanup(ctx, t, dst.Namespace)
				known[dst.Namespace] = struct{}{}
			}

			firstStepData := dataSteps[0]
			applyObjectNoCleanup(ctx, t, newSourceSecret(tcase.src, firstStepData))

			secretSync := &corev1alpha1.SecretSync{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "SecretSync",
					APIVersion: corev1alpha1.GroupVersion.String(),
				},
				Spec: corev1alpha1.SecretSyncSpec{
					Strategy: corev1alpha1.SecretSyncStrategy{
						Watch: &corev1alpha1.SecretSyncStrategyWatch{},
					},
					Src: corev1alpha1.NamespacedNameFromVanilla(tcase.src),
					Dest: func() []corev1alpha1.NamespacedName {
						out := []corev1alpha1.NamespacedName{}
						for _, dst := range tcase.dsts {
							out = append(out, corev1alpha1.NamespacedNameFromVanilla(dst))
						}
						return out
					}(),
				},
			}

			// Create SecretSync and wait for its Sync condition to be true.
			applyObjectWithCleanup(ctx, t, secretSync)
			require.NoError(t,
				Waiter.WaitForCondition(
					ctx, secretSync, corev1alpha1.SecretSyncSync, metav1.ConditionTrue, wait.WithTimeout(time.Second*10)))

			// Assert that dst secrets have correct data.
			for _, dst := range tcase.dsts {
				assertAPISecretData(ctx, t, dst, firstStepData)
			}

			// Iteratively update src data in steps and assert that new data is synced to all dst secrets.
			for _, stepData := range dataSteps[1:] {
				// Update data in src secret.
				applyObjectNoCleanup(ctx, t, newSourceSecret(tcase.src, stepData))

				// Wait for dst secrets to become synced (updated with new data) + assert shape of updated data.
				for _, dst := range tcase.dsts {
					require.NoError(t, Waiter.WaitForObject(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: dst.Namespace,
							Name:      dst.Name,
						},
					}, "become synchronized", func(obj client.Object) (done bool, err error) {
						s, ok := obj.(*corev1.Secret)
						require.True(t, ok)
						return reflect.DeepEqual(convertByteToStringMap(s.Data), stepData), nil
					}))

					assertAPISecretData(ctx, t, dst, stepData)
				}
			}

			// Delete SecretSync and wait for it to be gone.
			require.NoError(t, Client.Delete(ctx, secretSync))
			require.NoError(t, Waiter.WaitToBeGone(ctx, secretSync, func(_ client.Object) (done bool, err error) {
				return false, nil
			}))

			// Assert that all dst secrets are gone, too.
			for _, dst := range tcase.dsts {
				assertSecretNotFound(ctx, t, dst)
			}
		})
	}
}
