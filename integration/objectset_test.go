package integration

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/mt-sre/devkube/dev"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Simple Setup and Teardown test.
func TestObjectSet_setupTeardown(t *testing.T) {
	cm4 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cm-4",
			Labels: map[string]string{"test.package-operator.run/test": "True"},
		},
	}
	cmGVK, err := apiutil.GVKForObject(cm4, Scheme)
	require.NoError(t, err)
	cm4.SetGroupVersionKind(cmGVK)

	cm5 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cm-5",
		},
	}
	cm5.SetGroupVersionKind(cmGVK)

	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-setup-teardown",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name: "phase-1",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: runtime.RawExtension{
									Object: cm4,
								},
							},
						},
					},
					{
						Name: "phase-2",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: runtime.RawExtension{
									Object: cm5,
								},
							},
						},
					},
				},
				AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
					{
						Selector: corev1alpha1.ProbeSelector{
							Kind: &corev1alpha1.PackageProbeKindSpec{
								Kind: "ConfigMap",
							},
							Selector: &corev1alpha1.PackageProbeSelectorSpec{
								Selector: metav1.LabelSelector{
									MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
								},
							},
						},
						Probes: []corev1alpha1.Probe{
							{
								FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
									FieldA: ".metadata.name",
									FieldB: ".metadata.annotations.name",
								},
							},
						},
					},
				},
			},
		},
	}

	// TODO: Refactor in devkube:
	// ctx := logr.NewContext(context.Background(), testr.New(t))
	ctx := dev.ContextWithLogger(context.Background(), testr.New(t))

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	// ------------------------
	// Test phased setup logic.
	// ------------------------

	// Wait for false status to be reported.
	// Phase-1 is expected to fail because of the availabilityProbe.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionFalse))
	availableCond := meta.FindStatusCondition(objectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
	require.NotNil(t, availableCond, "Available condition is expected to be reported")
	assert.Equal(t, "ProbeFailure", availableCond.Reason)

	// expect cm-4 to be present.
	currentCM4 := &corev1.ConfigMap{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm4.Name, Namespace: objectSet.Namespace}, currentCM4))

	// expect cm-5 to NOT be present as Phase-1 didn't complete.
	currentCM5 := &corev1.ConfigMap{}
	require.EqualError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm5.Name, Namespace: objectSet.Namespace}, currentCM5), `configmaps "cm-5" not found`)

	// Patch cm-4 to pass probe.
	require.NoError(t,
		Client.Patch(ctx, currentCM4,
			client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"annotations":{"name":"cm-4"}}}`))))

	// Expect ObjectSet to become Available.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

	// Expect cm-5 to be present now.
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm5.Name, Namespace: objectSet.Namespace}, currentCM5))

	// ---------------------------
	// Test phased teardown logic.
	// ---------------------------

	// Patch cm-4 with extra finalizer, so it can't be deleted till we say so.
	require.NoError(t,
		Client.Patch(ctx, currentCM4,
			client.RawPatch(types.StrategicMergePatchType, []byte(`{"metadata":{"finalizers":["package-operator.run/test-blocker"]}}`))))

	// Archive ObjectSet to start teardown.
	require.NoError(t, Client.Patch(ctx, objectSet,
		client.RawPatch(types.MergePatchType, []byte(`{"spec":{"lifecycleState":"Archived"}}`))))

	// ObjectSet is Archiving...
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetArchived, metav1.ConditionFalse))

	// expect cm-5 to be gone already.
	require.EqualError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm5.Name, Namespace: objectSet.Namespace}, currentCM5), `configmaps "cm-5" not found`)

	// Remove our finalizer.
	require.NoError(t,
		Client.Patch(ctx, currentCM4,
			client.RawPatch(types.JSONPatchType, []byte(`[{"op":"remove","path":"/metadata/finalizers/0" }]`))))

	// ObjectSet is now Archived.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetArchived, metav1.ConditionTrue))

	// expect cm-4 to be also gone.
	require.EqualError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm4.Name, Namespace: objectSet.Namespace}, currentCM4), `configmaps "cm-4" not found`)
}

func TestObjectSet_handover(t *testing.T) {
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cm-1",
			Labels: map[string]string{"test.package-operator.run/test": "True"},
		},
	}
	cmGVK, err := apiutil.GVKForObject(cm1, Scheme)
	require.NoError(t, err)
	cm1.SetGroupVersionKind(cmGVK)

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cm-2",
		},
	}
	cm2.SetGroupVersionKind(cmGVK)

	objectSetRev1 := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rev1",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name: "phase-1",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: runtime.RawExtension{
									Object: cm1,
								},
							},
						},
					},
					{
						Name: "phase-2",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: runtime.RawExtension{
									Object: cm2,
								},
							},
						},
					},
				},
				AvailabilityProbes: []corev1alpha1.ObjectSetProbe{},
			},
		},
	}

	cm3 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cm-3",
		},
	}
	cm3.SetGroupVersionKind(cmGVK)

	objectSetRev2 := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rev2",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			Previous: []corev1alpha1.PreviousRevisionReference{
				{
					Name:  objectSetRev1.Name,
					Kind:  "ObjectSet",
					Group: corev1alpha1.GroupVersion.Group,
				},
			},
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name: "phase-1",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: runtime.RawExtension{
									Object: cm3, // replaces cm2
								},
							},
						},
					},
					{
						Name: "phase-2",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: runtime.RawExtension{
									Object: cm1, // moved between phases
								},
							},
						},
					},
				},
				AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
					{
						Selector: corev1alpha1.ProbeSelector{
							Kind: &corev1alpha1.PackageProbeKindSpec{
								Kind: "ConfigMap",
							},
							Selector: &corev1alpha1.PackageProbeSelectorSpec{
								Selector: metav1.LabelSelector{
									MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
								},
							},
						},
						Probes: []corev1alpha1.Probe{
							{
								FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
									FieldA: ".metadata.name",
									FieldB: ".metadata.annotations.name",
								},
							},
						},
					},
				},
			},
		},
	}

	// TODO: Refactor in devkube:
	// ctx := logr.NewContext(context.Background(), testr.New(t))
	ctx := dev.ContextWithLogger(context.Background(), testr.New(t))

	require.NoError(t, Client.Create(ctx, objectSetRev1))
	cleanupOnSuccess(ctx, t, objectSetRev1)

	// Expect ObjectSet Rev1 to become Available.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSetRev1, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

	// expect cm-1 to be present.
	currentCM1 := &corev1.ConfigMap{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm1.Name, Namespace: objectSetRev1.Namespace}, currentCM1))

	// expect cm-2 to be present.
	currentCM2 := &corev1.ConfigMap{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm2.Name, Namespace: objectSetRev2.Namespace}, currentCM2))

	// Create Revision 2
	require.NoError(t, Client.Create(ctx, objectSetRev2))
	cleanupOnSuccess(ctx, t, objectSetRev2)

	// Expect ObjectSet Rev2 report not Available, due to failing probes.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSetRev2, corev1alpha1.ObjectSetAvailable, metav1.ConditionFalse))
	availableCond := meta.FindStatusCondition(objectSetRev2.Status.Conditions, corev1alpha1.ObjectSetAvailable)
	require.NotNil(t, availableCond, "Available condition is expected to be reported")
	assert.Equal(t, "ProbeFailure", availableCond.Reason)

	// expect cm-1 to still be present and now owned by Rev2.
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm1.Name, Namespace: objectSetRev1.Namespace}, currentCM1))
	currentCM1.GetOwnerReferences()
	assertIsController(t, objectSetRev2, currentCM1)

	// expect cm-2 to still be present. (will be cleaned up after Rev2 is Available)
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm2.Name, Namespace: objectSetRev2.Namespace}, currentCM2))

	// expect cm-3 to also be present now.
	currentCM3 := &corev1.ConfigMap{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm3.Name, Namespace: objectSetRev2.Namespace}, currentCM3))

	// Patch cm-1 to pass probe.
	require.NoError(t,
		Client.Patch(ctx, currentCM1,
			client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"annotations":{"name":"cm-1"}}}`))))

	// Expect ObjectSet Rev2 to become Available.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSetRev2, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

	// Archive ObjectSet Rev1
	require.NoError(t, Client.Patch(ctx, objectSetRev1,
		client.RawPatch(types.MergePatchType, []byte(`{"spec":{"lifecycleState":"Archived"}}`))))
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSetRev1, corev1alpha1.ObjectSetArchived, metav1.ConditionTrue))

	// expect cm-2 to be gone.
	require.EqualError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm2.Name, Namespace: objectSetRev1.Namespace}, currentCM2), `configmaps "cm-2" not found`)

	// expect cm-3 and cm-1 to be still present
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm1.Name, Namespace: objectSetRev2.Namespace}, currentCM1))
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm3.Name, Namespace: objectSetRev2.Namespace}, currentCM3))
}

func cleanupOnSuccess(ctx context.Context, t *testing.T, obj client.Object) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			_ = Client.Delete(ctx, obj)
		}
	})
}

func assertIsController(t *testing.T, owner, obj client.Object) {
	t.Helper()
	gvk, err := apiutil.GVKForObject(owner, Scheme)
	require.NoError(t, err)

	var found bool
	for _, ownerRef := range obj.GetOwnerReferences() {
		ownerRefGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		require.NoError(t, err)

		if ownerRef.Kind == gvk.Kind &&
			ownerRefGV.Group == gvk.Group &&
			ownerRef.Name == owner.GetName() &&
			ownerRef.Controller != nil && *ownerRef.Controller {
			found = true
		}
	}

	if !found {
		t.Errorf("%s %s not controller of %s", gvk, client.ObjectKeyFromObject(owner), client.ObjectKeyFromObject(obj))
	}
}
