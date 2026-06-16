//go:build integration_hypershift

package packageoperator

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Regression: collision protection was bypassed for remote phases because:
//  1. PhaseEngineFactory did not pass UnfilteredReader to boxcutter, forcing
//     the CreateCollisionError fallback path.
//  2. AddDynamicCacheLabel used client.Merge which sent the full desired object
//     as a JSON merge patch, overwriting the pre-existing object on the hosted
//     cluster and effectively adopting it.
func TestHyperShiftCollisionPreventionPreventUnowned(t *testing.T) {
	const (
		namespace = "default-pko-hs-hc"
		cmName    = "test-hs-collision-prevent-unowned-cm"
		osName    = "test-hs-collision-prevent-unowned"
	)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	require.NoError(t, initClients(ctx))

	hClient, _, err := hostedClusterHandlers()
	require.NoError(t, err)

	// Pre-create a ConfigMap on the hosted cluster.
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "default",
		},
		Data: map[string]string{"pre-existing": "data"},
	}
	require.NoError(t, hClient.Create(ctx, existing))
	t.Cleanup(func() {
		_ = hClient.Delete(context.Background(), existing)
	})

	// Build an ObjectSet on the management cluster whose remote phase
	// contains the same ConfigMap with CollisionProtection=Prevent.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "default",
		},
		Data: map[string]string{"desired": "value"},
	}
	cmGVK, err := apiutil.GVKForObject(cm, Scheme)
	require.NoError(t, err)
	cm.SetGroupVersionKind(cmGVK)
	cmObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	require.NoError(t, err)

	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      osName,
			Namespace: namespace,
		},
		Spec: corev1alpha1.ObjectSetSpec{
			Revision: 1,
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "hosted-cluster",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "Prevent",
								Object:              unstructured.Unstructured{Object: cmObj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	// Wait for the ObjectSet to settle with any Available condition.
	require.NoError(t,
		Waiter.WaitForObject(ctx, objectSet,
			"Available condition to be set",
			func(client.Object) (bool, error) {
				return meta.FindStatusCondition(
					objectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable,
				) != nil, nil
			},
			wait.WithTimeout(60*time.Second),
		),
	)

	// Collision protection must block adoption: Available must be False.
	availableCond := meta.FindStatusCondition(objectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
	require.NotNil(t, availableCond, "Available condition must be set")
	assert.Equal(t, metav1.ConditionFalse, availableCond.Status,
		"ObjectSet must not become Available when collision protection is Prevent "+
			"and a pre-existing object exists on the target cluster")

	// Verify the pre-existing ConfigMap on the hosted cluster was NOT overwritten.
	actual := &corev1.ConfigMap{}
	require.NoError(t, hClient.Get(ctx, client.ObjectKeyFromObject(existing), actual))
	assert.Equal(t, "data", actual.Data["pre-existing"],
		"pre-existing ConfigMap data must not be overwritten")
	assert.Empty(t, actual.Data["desired"],
		"desired data must not appear on the pre-existing ConfigMap")
}

// Regression: same scenario but with a controller-owned pre-existing object.
func TestHyperShiftCollisionPreventionPreventOwned(t *testing.T) {
	const (
		namespace = "default-pko-hs-hc"
		cmName    = "test-hs-collision-prevent-owned-cm"
		osName    = "test-hs-collision-prevent-owned"
	)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	require.NoError(t, initClients(ctx))

	hClient, _, err := hostedClusterHandlers()
	require.NoError(t, err)

	// Pre-create a controller-owned ConfigMap on the hosted cluster.
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                "foreign-uid",
				Kind:               "ForeignController",
				Name:               "foreign-ctrl",
				APIVersion:         "example.com/v1",
				BlockOwnerDeletion: new(true),
				Controller:         new(true),
			}},
		},
		Data: map[string]string{"pre-existing": "data"},
	}
	require.NoError(t, hClient.Create(ctx, existing))
	t.Cleanup(func() {
		_ = hClient.Delete(context.Background(), existing)
	})

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "default",
		},
		Data: map[string]string{"desired": "value"},
	}
	cmGVK, err := apiutil.GVKForObject(cm, Scheme)
	require.NoError(t, err)
	cm.SetGroupVersionKind(cmGVK)
	cmObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	require.NoError(t, err)

	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      osName,
			Namespace: namespace,
		},
		Spec: corev1alpha1.ObjectSetSpec{
			Revision: 1,
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "hosted-cluster",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "Prevent",
								Object:              unstructured.Unstructured{Object: cmObj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	// Wait for the ObjectSet to settle with any Available condition.
	require.NoError(t,
		Waiter.WaitForObject(ctx, objectSet,
			"Available condition to be set",
			func(client.Object) (bool, error) {
				return meta.FindStatusCondition(
					objectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable,
				) != nil, nil
			},
			wait.WithTimeout(60*time.Second),
		),
	)

	// Collision protection must block adoption: Available must be False.
	availableCond := meta.FindStatusCondition(objectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
	require.NotNil(t, availableCond, "Available condition must be set")
	assert.Equal(t, metav1.ConditionFalse, availableCond.Status,
		"ObjectSet must not become Available when collision protection is Prevent "+
			"and a controller-owned pre-existing object exists on the target cluster")

	// Verify the pre-existing ConfigMap was NOT modified.
	actual := &corev1.ConfigMap{}
	require.NoError(t, hClient.Get(ctx, client.ObjectKeyFromObject(existing), actual))
	assert.Equal(t, "data", actual.Data["pre-existing"],
		"pre-existing ConfigMap data must not be overwritten")
	assert.Len(t, actual.OwnerReferences, 1,
		"pre-existing ConfigMap must retain exactly its original owner reference")
	assert.Equal(t, "foreign-uid", string(actual.OwnerReferences[0].UID),
		"pre-existing ConfigMap owner reference must not change")
}
