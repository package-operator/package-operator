//go:build integration

package packageoperator

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
		require.NoError(t, Waiter.WaitToBeGone(ctx, obj, func(_ client.Object) (done bool, err error) {
			return false, nil
		}))
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
		require.NoError(t, Waiter.WaitToBeGone(ctx, ns, func(_ client.Object) (done bool, err error) {
			return false, nil
		}))
	})
}

func assertAPISecretDataAndMutable(
	ctx context.Context, t *testing.T,
	key types.NamespacedName, expected map[string]string,
) {
	t.Helper()

	secret := &corev1.Secret{}
	require.NoError(t, Client.Get(ctx, key, secret))
	// All destination secrets must be mutable.
	assert.True(t, secret.Immutable == nil || *secret.Immutable == false)
	assert.Equal(t, expected, convertByteToStringMap(secret.Data))
}

func TestSecretSync_ForcesMutableDstSecrets(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	src := types.NamespacedName{
		Name:      "secretsync-src-1",
		Namespace: "default",
	}
	dst := types.NamespacedName{
		Name:      "secretsync-dst-1",
		Namespace: "default",
	}
	data := map[string]string{
		"hi": "there",
	}

	secretSync := &corev1alpha1.SecretSync{
		ObjectMeta: metav1.ObjectMeta{
			Name: "integration-immutable-src",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecretSync",
			APIVersion: corev1alpha1.GroupVersion.String(),
		},
		Spec: corev1alpha1.SecretSyncSpec{
			Strategy: corev1alpha1.SecretSyncStrategy{
				Poll: &corev1alpha1.SecretSyncStrategyPoll{
					Interval: metav1.Duration{Duration: time.Second},
				},
			},
			Src: corev1alpha1.NamespacedNameFromVanilla(src),
			Dest: []corev1alpha1.NamespacedName{
				corev1alpha1.NamespacedNameFromVanilla(dst),
			},
		},
	}

	srcSecret := newSourceSecret(src, data)
	srcSecret.Immutable = ptr.To(true)
	applyObjectWithCleanup(ctx, t, srcSecret)
	applyObjectWithCleanup(ctx, t, secretSync)

	require.NoError(t,
		Waiter.WaitForCondition(
			ctx, secretSync, corev1alpha1.SecretSyncSync, metav1.ConditionTrue, wait.WithTimeout(time.Second*10)))

	assertAPISecretDataAndMutable(ctx, t, dst, data)
}

func TestSecretSync_Matrix(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	type tcase struct {
		name string
		// Src and Dst namespaces MUST not exist.
		// Dst namespaces are allowed to be non-unique within the list.
		src  types.NamespacedName
		dsts []types.NamespacedName

		// Will be populated by a for-loop below.
		stategy corev1alpha1.SecretSyncStrategy
	}

	// Each test case is combined with all strategies listed here:
	strategies := []func(tcase) tcase{
		func(t tcase) tcase {
			t.name += "_watch"
			t.stategy = corev1alpha1.SecretSyncStrategy{
				Watch: &corev1alpha1.SecretSyncStrategyWatch{},
			}
			return t
		},
		func(t tcase) tcase {
			t.name += "_poll"
			t.stategy = corev1alpha1.SecretSyncStrategy{
				Poll: &corev1alpha1.SecretSyncStrategyPoll{
					Interval: metav1.Duration{Duration: time.Second},
				},
			}
			return t
		},
	}

	tcases := []tcase{
		{
			name: "MultipleTargetsInSingleNamespace",
			src:  types.NamespacedName{Namespace: "secretsync-src-1", Name: "src-simple"},
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

	generatedTestCases := []tcase{}
	for _, tcase := range tcases {
		for _, applyStrategy := range strategies {
			tc := applyStrategy(tcase)
			generatedTestCases = append(generatedTestCases, tc)
		}
	}

	dataSteps := []map[string]string{
		{"foo": "bar"},
		{"foo": "two", "banana": "dance"},
		{"foo": "three", "banana": "dance", "socken": "affe"},
	}

	for _, tcase := range generatedTestCases {
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
					Name: "integration-matrix",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "SecretSync",
					APIVersion: corev1alpha1.GroupVersion.String(),
				},
				Spec: corev1alpha1.SecretSyncSpec{
					Strategy: tcase.stategy,
					Src:      corev1alpha1.NamespacedNameFromVanilla(tcase.src),
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
				assertAPISecretDataAndMutable(ctx, t, dst, firstStepData)
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

					assertAPISecretDataAndMutable(ctx, t, dst, stepData)
				}
			}

			// Delete SecretSync and wait for it to be gone.
			require.NoError(t, Client.Delete(ctx, secretSync))
			require.NoError(t, Waiter.WaitToBeGone(ctx, secretSync, func(_ client.Object) (done bool, err error) {
				return false, nil
			}))

			// Assert that all dst secrets vanish, too.
			var eg errgroup.Group
			for _, dst := range tcase.dsts {
				eg.Go(func() error {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: dst.Namespace,
							Name:      dst.Name,
						},
					}
					return Waiter.WaitToBeGone(ctx, secret, func(_ client.Object) (done bool, err error) {
						return false, nil
					})
				})
			}

			require.NoError(t, eg.Wait())
		})
	}
}
