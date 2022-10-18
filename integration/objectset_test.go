package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/mt-sre/devkube/dev"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Simple Setup, Pause and Teardown test.
//
//nolint:maintidx
func TestObjectSet_setupPauseTeardown(t *testing.T) {
	defaultObjectSet := func(cm4, cm5 *corev1.ConfigMap) (*corev1alpha1.ObjectSet, error) {
		cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm4)
		if err != nil {
			return nil, err
		}
		cm5Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm5)
		if err != nil {
			return nil, err
		}
		return &corev1alpha1.ObjectSet{
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
									Object: unstructured.Unstructured{Object: cm4Obj},
								},
							},
						},
						{
							Name: "phase-2",
							Objects: []corev1alpha1.ObjectSetObject{
								{
									Object: unstructured.Unstructured{Object: cm5Obj},
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
								Selector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
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
		}, nil
	}

	tests := []struct {
		name      string
		objectSet func(cm4, cm5 *corev1.ConfigMap) (*corev1alpha1.ObjectSet, error)
	}{
		{
			name:      "without phase class",
			objectSet: defaultObjectSet,
		},
		{
			name: "with phase class",
			objectSet: func(cm4, cm5 *corev1.ConfigMap) (*corev1alpha1.ObjectSet, error) {
				objectSet, err := defaultObjectSet(cm4, cm5)
				if err != nil {
					return nil, err
				}
				objectSet.Spec.Phases[0].Class = "default"
				objectSet.Spec.Phases[1].Class = "default"
				return objectSet, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cm4 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "cm-4",
					Labels: map[string]string{"test.package-operator.run/test": "True"},
				},
				Data: map[string]string{
					"banana": "bread",
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

			objectSet, err := test.objectSet(cm4, cm5)
			require.NoError(t, err)

			cm4Key := client.ObjectKey{
				Name: cm4.Name, Namespace: objectSet.Namespace}
			cm5Key := client.ObjectKey{
				Name: cm5.Name, Namespace: objectSet.Namespace}

			ctx := logr.NewContext(context.Background(), testr.New(t))

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

			// expect cm-4 to be reported under "ControllerOf"
			require.Equal(t, []corev1alpha1.ControlledObjectReference{
				{
					Kind:      "ConfigMap",
					Name:      cm4.Name,
					Namespace: "default",
				},
			}, objectSet.Status.ControllerOf)

			// expect Succeeded condition to be not present
			succeededCond := meta.FindStatusCondition(objectSet.Status.Conditions, corev1alpha1.ObjectSetSucceeded)
			require.Nil(t, succeededCond, "expected Succeeded condition to not be reported")

			// expect InTransition to be reported and True
			inTransition := meta.IsStatusConditionTrue(objectSet.Status.Conditions, corev1alpha1.ObjectSetInTransition)
			require.True(t, inTransition, "expected InTransition to be reported and True")

			// expect cm-4 to be present.
			currentCM4 := &corev1.ConfigMap{}
			require.NoError(t, Client.Get(ctx, cm4Key, currentCM4))

			// expect cm-5 to NOT be present as Phase-1 didn't complete.
			currentCM5 := &corev1.ConfigMap{}
			require.EqualError(t, Client.Get(ctx, cm5Key, currentCM5), `configmaps "cm-5" not found`)

			// Patch cm-4 to pass probe.
			// -------------------------
			require.NoError(t,
				Client.Patch(ctx, currentCM4,
					client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"annotations":{"name":"cm-4"}}}`))))

			// Expect ObjectSet to become Available.
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

			// expect Succeeded condition to be True
			isSucceeded := meta.IsStatusConditionTrue(objectSet.Status.Conditions, corev1alpha1.ObjectSetSucceeded)
			require.True(t, isSucceeded, "expected Succeeded condition to be True")

			// expect InTransition condition to be not present
			inTransitionCond := meta.FindStatusCondition(objectSet.Status.Conditions, corev1alpha1.ObjectSetInTransition)
			require.Nil(t, inTransitionCond, "expected InTransition condition to not be reported")

			// expect cm-4 and cm-5 to be reported under "ControllerOf"
			require.Equal(t, []corev1alpha1.ControlledObjectReference{
				{
					Kind:      "ConfigMap",
					Name:      currentCM4.Name,
					Namespace: currentCM4.Namespace,
				},
				{
					Kind:      "ConfigMap",
					Name:      cm5.Name,
					Namespace: "default",
				},
			}, objectSet.Status.ControllerOf)

			// Expect cm-5 to be present now.
			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: cm5.Name, Namespace: objectSet.Namespace}, currentCM5))

			// -----------
			// Test pause.
			// -----------

			// Pause ObjectSet.
			require.NoError(t, Client.Patch(ctx, objectSet,
				client.RawPatch(types.MergePatchType, []byte(`{"spec":{"lifecycleState":"Paused"}}`))))

			// ObjectSet is Pausing...
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetPaused, metav1.ConditionTrue))

			// should be reconciled to "banana":"bread", if reconciler would not be paused.
			require.NoError(t, Client.Patch(ctx, currentCM4,
				client.RawPatch(types.MergePatchType, []byte(`{"data":{"banana":"toast"}}`))))

			// Wait 5s for the object to be reconciled, which should not happen, because it's paused.
			require.EqualError(t,
				Waiter.WaitForObject(ctx, currentCM4, "to NOT be reconciled to its desired state", func(obj client.Object) (done bool, err error) {
					cm := obj.(*corev1.ConfigMap)
					return cm.Data["banana"] == "bread", nil
				}, dev.WithTimeout(5*time.Second)), wait.ErrWaitTimeout.Error())

			// Unpause ObjectSet.
			require.NoError(t, Client.Patch(ctx, objectSet,
				client.RawPatch(types.MergePatchType, []byte(`{"spec":{"lifecycleState":"Active"}}`))))

			// ObjectSet is Unpausing...
			require.NoError(t,
				Waiter.WaitForObject(ctx, objectSet, "to not report Paused anymore", func(obj client.Object) (done bool, err error) {
					os := obj.(*corev1alpha1.ObjectSet)
					pausedCond := meta.FindStatusCondition(os.Status.Conditions, corev1alpha1.ObjectSetPaused)
					return pausedCond == nil, nil
				}))

			// Wait 10s for the object to be reconciled, which should now happen!
			require.NoError(t,
				Waiter.WaitForObject(ctx, currentCM4, "to be reconciled to its desired state", func(obj client.Object) (done bool, err error) {
					cm := obj.(*corev1.ConfigMap)
					return cm.Data["banana"] == "bread", nil
				}, dev.WithTimeout(10*time.Second)))

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

			// expect no "ControllerOf" left
			require.Len(t, objectSet.Status.ControllerOf, 0)
		})
	}
}

// nolint:maintidx
func TestObjectSet_handover(t *testing.T) {
	defaultObjectSetRev1 := func(cm1, cm2 *corev1.ConfigMap) (*corev1alpha1.ObjectSet, error) {
		cm1Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm1)
		if err != nil {
			return nil, err
		}
		cm2Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm2)
		if err != nil {
			return nil, err
		}
		return &corev1alpha1.ObjectSet{
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
									Object: unstructured.Unstructured{Object: cm1Obj},
								},
							},
						},
						{
							Name: "phase-2",
							Objects: []corev1alpha1.ObjectSetObject{
								{
									Object: unstructured.Unstructured{Object: cm2Obj},
								},
							},
						},
					},
					AvailabilityProbes: []corev1alpha1.ObjectSetProbe{},
				},
			},
		}, nil
	}
	defaultObjectSetRev2 := func(cm1, cm3 *corev1.ConfigMap, rev1 *corev1alpha1.ObjectSet) (*corev1alpha1.ObjectSet, error) {
		cm1Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm1)
		if err != nil {
			return nil, err
		}
		cm3Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm3)
		if err != nil {
			return nil, err
		}
		return &corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rev2",
				Namespace: "default",
			},
			Spec: corev1alpha1.ObjectSetSpec{
				Previous: []corev1alpha1.PreviousRevisionReference{
					{
						Name: rev1.Name,
					},
				},
				ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
					Phases: []corev1alpha1.ObjectSetTemplatePhase{
						{
							Name: "phase-1",
							Objects: []corev1alpha1.ObjectSetObject{
								{
									Object: unstructured.Unstructured{Object: cm3Obj}, // replaces cm2
								},
							},
						},
						{
							Name: "phase-2",
							Objects: []corev1alpha1.ObjectSetObject{
								{
									Object: unstructured.Unstructured{Object: cm1Obj}, // moved between phases
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
								Selector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
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
		}, nil
	}

	tests := []struct {
		name          string
		objectSetRev1 func(cm1, cm2 *corev1.ConfigMap) (*corev1alpha1.ObjectSet, error)
		objectSetRev2 func(cm1, cm3 *corev1.ConfigMap, rev1 *corev1alpha1.ObjectSet) (*corev1alpha1.ObjectSet, error)
	}{
		{
			name:          "without phase class",
			objectSetRev1: defaultObjectSetRev1,
			objectSetRev2: defaultObjectSetRev2,
		},
		{
			name: "with phase class",
			objectSetRev1: func(cm1, cm2 *corev1.ConfigMap) (*corev1alpha1.ObjectSet, error) {
				objectSet, err := defaultObjectSetRev1(cm1, cm2)
				if err != nil {
					return nil, err
				}
				objectSet.Spec.Phases[0].Class = "default"
				objectSet.Spec.Phases[1].Class = "default"
				return objectSet, nil
			},
			objectSetRev2: func(cm1, cm3 *corev1.ConfigMap, rev1 *corev1alpha1.ObjectSet) (*corev1alpha1.ObjectSet, error) {
				objectSet, err := defaultObjectSetRev2(cm1, cm3, rev1)
				if err != nil {
					return nil, err
				}
				objectSet.Spec.Phases[0].Class = "default"
				objectSet.Spec.Phases[1].Class = "default"
				return objectSet, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

			objectSetRev1, err := test.objectSetRev1(cm1, cm2)
			require.NoError(t, err)

			cm3 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cm-3",
				},
			}
			cm3.SetGroupVersionKind(cmGVK)

			objectSetRev2, err := test.objectSetRev2(cm1, cm3, objectSetRev1)
			require.NoError(t, err)

			ctx := logr.NewContext(context.Background(), testr.New(t))

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

			// expect cm-1 and cm-2 to be reported under "ControllerOf" in revision 1
			require.Equal(t, []corev1alpha1.ControlledObjectReference{
				{
					Kind:      "ConfigMap",
					Name:      currentCM1.Name,
					Namespace: currentCM1.Namespace,
				},
				{
					Kind:      "ConfigMap",
					Name:      currentCM2.Name,
					Namespace: currentCM2.Namespace,
				},
			}, objectSetRev1.Status.ControllerOf)

			// Create Revision 2
			require.NoError(t, Client.Create(ctx, objectSetRev2))
			cleanupOnSuccess(ctx, t, objectSetRev2)

			// Expect ObjectSet Rev2 report not Available, due to failing probes on cm-1.
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectSetRev2, corev1alpha1.ObjectSetAvailable, metav1.ConditionFalse))
			availableCond := meta.FindStatusCondition(objectSetRev2.Status.Conditions, corev1alpha1.ObjectSetAvailable)
			require.NotNil(t, availableCond, "Available condition is expected to be reported")
			assert.Equal(t, "ProbeFailure", availableCond.Reason)

			// expect cm-1 to still be present and now owned by Rev2.
			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: cm1.Name, Namespace: objectSetRev1.Namespace}, currentCM1))
			assertControllerNameHasPrefix(t, objectSetRev2.Name, currentCM1)

			// expect cm-2 to still be present.
			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: cm2.Name, Namespace: objectSetRev2.Namespace}, currentCM2))

			// expect cm-3 to also be present now.
			currentCM3 := &corev1.ConfigMap{}
			require.NoError(t, Client.Get(ctx, client.ObjectKey{
				Name: cm3.Name, Namespace: objectSetRev2.Namespace}, currentCM3))

			// wait for Revision 1 to report "InTransition" (needed to ensure that the next assertions are not racy)
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectSetRev1, corev1alpha1.ObjectSetInTransition, metav1.ConditionTrue))

			// expect only cm-2 to be reported under "ControllerOf" in revision 1
			require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(objectSetRev1), objectSetRev1))
			require.Equal(t, []corev1alpha1.ControlledObjectReference{
				{
					Kind:      "ConfigMap",
					Name:      currentCM2.Name,
					Namespace: currentCM2.Namespace,
				},
			}, objectSetRev1.Status.ControllerOf)

			// expect cm-1 and cm-3 to be reported under "ControllerOf" in revision 2
			require.Equal(t, []corev1alpha1.ControlledObjectReference{
				{
					Kind:      "ConfigMap",
					Name:      currentCM3.Name,
					Namespace: currentCM3.Namespace,
				},
				{
					Kind:      "ConfigMap",
					Name:      currentCM1.Name,
					Namespace: currentCM1.Namespace,
				},
			}, objectSetRev2.Status.ControllerOf)

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
		})
	}

}

func cleanupOnSuccess(ctx context.Context, t *testing.T, obj client.Object) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			_ = Client.Delete(ctx, obj)
		}
	})
}

func assertControllerNameHasPrefix(t *testing.T, ownerNamePrefix string, obj client.Object) {
	t.Helper()
	found := controllerNameHasPrefix(ownerNamePrefix, obj)

	if !found {
		t.Errorf("controller name of %s not prefixed with %s", client.ObjectKeyFromObject(obj), ownerNamePrefix)
	}
}

func controllerNameHasPrefix(ownerNamePrefix string, obj client.Object) bool {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.Controller == nil || !*ownerRef.Controller {
			continue
		}
		if strings.HasPrefix(ownerRef.Name, ownerNamePrefix) {
			return true
		}
	}
	return false
}
