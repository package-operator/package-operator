//go:build integration || integration_hypershift

package packageoperator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"pkg.package-operator.run/cardboard/kubeutils/wait"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachinerywait "k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// assertControllerNameHasPrefix can be used to check if an object is controlled by an owner object, or if it
// is controlled by an object, which is controlled by an owner object. For example, given a configMap, you can
// check if the ObjectSetPhase which controls the configMap is controlled by an ObjectSet.
func assertControllerNameHasPrefix(t *testing.T, ownerNamePrefix string, obj client.Object) {
	t.Helper()
	found := controllerNameHasPrefix(ownerNamePrefix, obj)

	require.True(t, found, "controller name of %s not prefixed with %s", client.ObjectKeyFromObject(obj), ownerNamePrefix)
}

func someOwnerRefSatisfiesCond(obj client.Object, filter, cond func(ownerRef metav1.OwnerReference) bool) bool {
	ownerRefs := append(getOwnerRefsFromAnnotations(obj), obj.GetOwnerReferences()...)
	for _, ownerRef := range ownerRefs {
		if !filter(ownerRef) {
			continue
		}
		if cond(ownerRef) {
			return true
		}
	}
	return false
}

func ownerNameHasPrefix(ownerNamePrefix string, obj client.Object) bool {
	return someOwnerRefSatisfiesCond(obj,
		// filter: noop
		func(_ metav1.OwnerReference) bool {
			return true
		},
		// condition:
		func(ownerRef metav1.OwnerReference) bool {
			return strings.HasPrefix(ownerRef.Name, ownerNamePrefix)
		},
	)
}

func controllerNameHasPrefix(controllerNamePrefix string, obj client.Object) bool {
	return someOwnerRefSatisfiesCond(obj,
		// filter: only ownerReferences pointing to the controlling object
		func(ownerRef metav1.OwnerReference) bool {
			return ownerRef.Controller != nil && *ownerRef.Controller
		},
		// condition:
		func(ownerRef metav1.OwnerReference) bool {
			return strings.HasPrefix(ownerRef.Name, controllerNamePrefix)
		},
	)
}

func getOwnerRefsFromAnnotations(obj client.Object) []metav1.OwnerReference {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}
	if len(annotations["package-operator.run/owners"]) == 0 {
		return nil
	}

	var ownerReferences []metav1.OwnerReference
	if err := json.Unmarshal([]byte(annotations["package-operator.run/owners"]), &ownerReferences); err != nil {
		panic(err)
	}
	return ownerReferences
}

func cleanupOnSuccess(ctx context.Context, t *testing.T, obj client.Object) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			// Make sure objects are completely gone before closing the test.
			_ = Client.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground))
			_ = Waiter.WaitToBeGone(ctx, obj, func(client.Object) (bool, error) { return false, nil })
		}
	})
}

func defaultObjectSet(cm4, cm5 *corev1.ConfigMap, namespace, class string) (*corev1alpha1.ObjectSet, error) {
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
			Namespace: namespace,
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: class,
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
					{
						Name:  "phase-2",
						Class: class,
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

func defaultObjectSetRev1(cm1, cm2 *corev1.ConfigMap, namespace, class string) (*corev1alpha1.ObjectSet, error) {
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
			Namespace: namespace,
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: class,
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: unstructured.Unstructured{Object: cm1Obj},
							},
						},
					},
					{
						Name:  "phase-2",
						Class: class,
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: unstructured.Unstructured{Object: cm2Obj},
							},
						},
					},
				},
				AvailabilityProbes: []corev1alpha1.ObjectSetProbe{},
			},
			Revision: 1,
		},
	}, nil
}

func defaultObjectSetRev2(
	cm1, cm3 *corev1.ConfigMap, rev1 *corev1alpha1.ObjectSet, namespace, class string,
) (*corev1alpha1.ObjectSet, error) {
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
			Namespace: namespace,
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
						Name:  "phase-1",
						Class: class,
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: unstructured.Unstructured{Object: cm3Obj}, // replaces cm2
							},
						},
					},
					{
						Name:  "phase-2",
						Class: class,
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
			Revision: 2,
		},
	}, nil
}

func objectsSetPhaseKey(objectSet *corev1alpha1.ObjectSet) client.ObjectKey {
	return client.ObjectKey{
		Name:      objectSet.Name + "-" + objectSet.Spec.ObjectSetTemplateSpec.Phases[0].Name,
		Namespace: objectSet.Namespace,
	}
}

func objectSetPhaseTestHelper(
	ctx context.Context, t *testing.T, objectSet *corev1alpha1.ObjectSet, cm *corev1.ConfigMap,
) {
	t.Helper()

	require.NotNil(t, objectSet)
	require.NotNil(t, cm)

	// expect objectSetPhase to be present
	objectSetPhase := &corev1alpha1.ObjectSetPhase{}
	require.NoError(t, Client.Get(ctx, objectsSetPhaseKey(objectSet), objectSetPhase))

	// delete objectSetPhase with orphan cascade
	require.NoError(t, Client.Delete(ctx, objectSetPhase, client.PropagationPolicy(metav1.DeletePropagationOrphan)))
	err := Waiter.WaitToBeGone(ctx, objectSetPhase, func(client.Object) (bool, error) { return false, nil })
	require.NoError(t, err)

	// expect cm to still be there
	currentCM := &corev1.ConfigMap{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: cm.Name, Namespace: objectSet.Namespace,
	}, currentCM))
}

func runObjectSetOrphanCascadeDeletionTest(t *testing.T, namespace, class string) { //nolint:thelper
	runObjectSetOrphanCascadeDeletionTestWithCustomHandlers(t, Client, Waiter, namespace, class)
}

// destClient and destWaiter allow to monitor the status of the ObjectSet resources in a different cluster
// than the one where ObjectSets are created, like in HyperShift integration tests.
func runObjectSetOrphanCascadeDeletionTestWithCustomHandlers(
	t *testing.T, destClient client.Client, _ *wait.Waiter, namespace, class string,
) {
	t.Helper()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: namespace,
			Labels:    map[string]string{"test.package-operator.run/test": "True"},
		},
	}
	cmGVK, err := apiutil.GVKForObject(cm, Scheme)
	require.NoError(t, err)
	cm.SetGroupVersionKind(cmGVK)

	objectSet, err := simpleObjectSet(cm, namespace, class)
	require.NoError(t, err)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	cleanupOnSuccess(ctx, t, cm)

	require.NoError(t, Client.Create(ctx, objectSet))

	// Expect ObjectSet to become Available.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

	// expect cm to be present.
	currentCM := &corev1.ConfigMap{}
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm.Name, Namespace: objectSet.Namespace,
	}, currentCM))

	// expect cm to be reported under "ControllerOf"
	require.Equal(t, []corev1alpha1.ControlledObjectReference{
		{
			Kind:      "ConfigMap",
			Name:      currentCM.Name,
			Namespace: currentCM.Namespace,
		},
	}, objectSet.Status.ControllerOf)

	// delete objectSet with orphan cascade
	require.NoError(t, Client.Delete(ctx, objectSet, client.PropagationPolicy(metav1.DeletePropagationOrphan)))
	err = Waiter.WaitToBeGone(ctx, objectSet, func(client.Object) (bool, error) { return false, nil })
	require.NoError(t, err)

	// expect objectSet not to be present anymore
	currentObjectSet := &corev1alpha1.ObjectSet{}
	require.EqualError(t, Client.Get(ctx, client.ObjectKey{
		Name: objectSet.Name, Namespace: objectSet.Namespace,
	}, currentObjectSet), fmt.Sprintf(`objectsets.package-operator.run "%s" not found`, objectSet.Name))

	// expect cm to still be there
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm.Name, Namespace: objectSet.Namespace,
	}, currentCM))

	// run objectSetPhase checks if an objectSetPhase object is present
	if class != "" {
		objectSetPhaseTestHelper(ctx, t, objectSet, cm)
	}
}

func runObjectSetHandoverTest(t *testing.T, namespace, class string) { //nolint:thelper
	runObjectSetHandoverTestWithCustomHandlers(t, Client, Waiter, namespace, class)
}

// destClient and destWaiter allow to monitor the status of the ObjectSet resources in a different cluster
// than the one where ObjectSets are created, like in HyperShift integration tests.
func runObjectSetHandoverTestWithCustomHandlers(
	t *testing.T, destClient client.Client, _ *wait.Waiter, namespace, class string,
) {
	t.Helper()

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

	objectSetRev1, err := defaultObjectSetRev1(cm1, cm2, namespace, class)
	require.NoError(t, err)

	cm3 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cm-3",
		},
	}
	cm3.SetGroupVersionKind(cmGVK)

	objectSetRev2, err := defaultObjectSetRev2(cm1, cm3, objectSetRev1, namespace, class)
	require.NoError(t, err)

	ctx := logr.NewContext(context.Background(), testr.New(t))

	require.NoError(t, Client.Create(ctx, objectSetRev1))
	cleanupOnSuccess(ctx, t, objectSetRev1)

	// Expect ObjectSet Rev1 to become Available.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSetRev1, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

	// expect cm-1 to be present.
	currentCM1 := &corev1.ConfigMap{}
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm1.Name, Namespace: objectSetRev1.Namespace,
	}, currentCM1))

	// expect cm-2 to be present.
	currentCM2 := &corev1.ConfigMap{}
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm2.Name, Namespace: objectSetRev2.Namespace,
	}, currentCM2))

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

	// expect cm-2 to still be present.
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm2.Name, Namespace: objectSetRev2.Namespace,
	}, currentCM2))

	// expect cm-3 to also be present now.
	currentCM3 := &corev1.ConfigMap{}
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm3.Name, Namespace: objectSetRev2.Namespace,
	}, currentCM3))

	// wait for Revision 1 to report "InTransition" (needed to ensure that the next assertions are not racy)
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSetRev1, corev1alpha1.ObjectSetInTransition, metav1.ConditionTrue))

	// expect cm-1 to still be present and now controlled by Rev2.
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm1.Name, Namespace: objectSetRev1.Namespace,
	}, currentCM1))

	assertControllerNameHasPrefix(t, objectSetRev2.Name, currentCM1)

	// expect only cm-2 to be reported under "ControllerOf" in revision 1
	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(objectSetRev1), objectSetRev1))
	require.Equal(t, []corev1alpha1.ControlledObjectReference{
		{
			Kind:      "ConfigMap",
			Name:      currentCM2.Name,
			Namespace: currentCM2.Namespace,
		},
	}, objectSetRev1.Status.ControllerOf)

	// Wait for cm-1 and cm-3 to be reported under "ControllerOf" in revision 2
	expectedControllerOf := []corev1alpha1.ControlledObjectReference{
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
	}
	require.NoError(t, Waiter.WaitForObject(ctx, objectSetRev2,
		"Waiting for .status.controllerOf to be updated",
		func(client.Object) (bool, error) {
			return reflect.DeepEqual(objectSetRev2.Status.ControllerOf, expectedControllerOf), nil
		}))
	require.Equal(t, expectedControllerOf, objectSetRev2.Status.ControllerOf)

	// Patch cm-1 to pass probe.
	require.NoError(t,
		destClient.Patch(ctx, currentCM1,
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
	require.EqualError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm2.Name, Namespace: objectSetRev1.Namespace,
	}, currentCM2), `configmaps "cm-2" not found`)

	// expect cm-3 and cm-1 to be still present
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm1.Name, Namespace: objectSetRev2.Namespace,
	}, currentCM1))
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm3.Name, Namespace: objectSetRev2.Namespace,
	}, currentCM3))
}

func runObjectSetSetupPauseTeardownTest(t *testing.T, namespace, class string) { //nolint:thelper
	runObjectSetSetupPauseTeardownTestWithCustomHandlers(t, Client, Waiter, namespace, class)
}

// destClient and destWaiter allow to monitor the status of the ObjectSet resources in a different cluster
// than the one where ObjectSets are created, like in HyperShift integration tests.
//
//nolint:thelper
func runObjectSetSetupPauseTeardownTestWithCustomHandlers(
	t *testing.T, destClient client.Client, destWaiter *wait.Waiter, namespace, class string,
) {
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

	objectSet, err := defaultObjectSet(cm4, cm5, namespace, class)
	require.NoError(t, err)

	cm4Key := client.ObjectKey{
		Name: cm4.Name, Namespace: objectSet.Namespace,
	}
	cm5Key := client.ObjectKey{
		Name: cm5.Name, Namespace: objectSet.Namespace,
	}

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
	expectedControllerOf := []corev1alpha1.ControlledObjectReference{
		{
			Kind:      "ConfigMap",
			Name:      cm4.Name,
			Namespace: namespace,
		},
	}
	require.NoError(t, Waiter.WaitForObject(ctx, objectSet,
		"Waiting for .status.controllerOf to be updated",
		func(client.Object) (bool, error) {
			return reflect.DeepEqual(objectSet.Status.ControllerOf, expectedControllerOf), nil
		}))

	// expect Succeeded condition to be not present
	succeededCond := meta.FindStatusCondition(objectSet.Status.Conditions, corev1alpha1.ObjectSetSucceeded)
	require.Nil(t, succeededCond, "expected Succeeded condition to not be reported")

	// expect InTransition to be reported and True
	inTransition := meta.IsStatusConditionTrue(objectSet.Status.Conditions, corev1alpha1.ObjectSetInTransition)
	require.True(t, inTransition, "expected InTransition to be reported and True")

	// expect cm-4 to be present.
	currentCM4 := &corev1.ConfigMap{}
	require.NoError(t, destClient.Get(ctx, cm4Key, currentCM4))

	// expect cm-5 to NOT be present as Phase-1 didn't complete.
	currentCM5 := &corev1.ConfigMap{}
	require.EqualError(t, destClient.Get(ctx, cm5Key, currentCM5), `configmaps "cm-5" not found`)

	// Patch cm-4 to pass probe.
	// -------------------------
	require.NoError(t,
		destClient.Patch(ctx, currentCM4,
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
			Namespace: namespace,
		},
	}, objectSet.Status.ControllerOf)

	// Expect cm-5 to be present now.
	require.NoError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm5.Name, Namespace: objectSet.Namespace,
	}, currentCM5))

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
	require.NoError(t, destClient.Patch(ctx, currentCM4,
		client.RawPatch(types.MergePatchType, []byte(`{"data":{"banana":"toast"}}`))))

	// Wait 5s for the object to be reconciled, which should not happen, because it's paused.
	err = destWaiter.WaitForObject(ctx, currentCM4, "to NOT be reconciled to its desired state",
		func(obj client.Object) (done bool, err error) {
			cm := obj.(*corev1.ConfigMap)
			return cm.Data["banana"] == "bread", nil
		}, wait.WithTimeout(5*time.Second))
	require.True(t, apimachinerywait.Interrupted(err))

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
	err = destWaiter.WaitForObject(ctx, currentCM4, "to be reconciled to its desired state",
		func(obj client.Object) (done bool, err error) {
			cm := obj.(*corev1.ConfigMap)
			return cm.Data["banana"] == "bread", nil
		}, wait.WithTimeout(10*time.Second))
	require.NoError(t, err)

	// ---------------------------
	// Test phased teardown logic.
	// ---------------------------

	// Patch cm-4 with extra finalizer, so it can't be deleted till we say so.
	err = destClient.Patch(ctx, currentCM4,
		client.RawPatch(types.StrategicMergePatchType,
			[]byte(`{"metadata":{"finalizers":["package-operator.run/test-blocker"]}}`),
		),
	)
	require.NoError(t, err)

	// Archive ObjectSet to start teardown.
	require.NoError(t, Client.Patch(ctx, objectSet,
		client.RawPatch(types.MergePatchType, []byte(`{"spec":{"lifecycleState":"Archived"}}`))))

	// ObjectSet is Archiving...
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetArchived, metav1.ConditionFalse))

	// expect cm-5 to be gone already.
	require.EqualError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm5.Name, Namespace: objectSet.Namespace,
	}, currentCM5), `configmaps "cm-5" not found`)

	// Remove our finalizer.
	require.NoError(t,
		destClient.Patch(ctx, currentCM4,
			client.RawPatch(types.JSONPatchType, []byte(`[{"op":"remove","path":"/metadata/finalizers/0" }]`))))

	// ObjectSet is now Archived.
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetArchived, metav1.ConditionTrue))

	// expect cm-4 to be also gone.
	require.EqualError(t, destClient.Get(ctx, client.ObjectKey{
		Name: cm4.Name, Namespace: objectSet.Namespace,
	}, currentCM4), `configmaps "cm-4" not found`)

	// expect no "ControllerOf" left
	require.Empty(t, objectSet.Status.ControllerOf)
}

func simpleObjectSet(cm *corev1.ConfigMap, namespace, class string) (*corev1alpha1.ObjectSet, error) {
	cmObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	if err != nil {
		return nil, err
	}
	return &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-orphan-cascade-delete",
			Namespace: namespace,
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: class,
						Objects: []corev1alpha1.ObjectSetObject{
							{
								Object: unstructured.Unstructured{Object: cmObj},
							},
						},
					},
				},
			},
		},
	}, nil
}

func filterListByOwningObjectPrefix[T client.Object, U []T](parentName string, list U) U {
	ownedByParent := make(U, 0)
	for _, obj := range list {
		if !ownerNameHasPrefix(parentName, obj) {
			continue
		}
		ownedByParent = append(ownedByParent, obj)
	}
	return ownedByParent
}

func mapToNamespacedNames[T client.Object, U []T](list U) []types.NamespacedName {
	names := make([]types.NamespacedName, 0, len(list))
	for _, obj := range list {
		names = append(names, types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		})
	}
	return names
}

func getObjectSetNamesControlledBy(ctx context.Context, parent types.NamespacedName) ([]types.NamespacedName, error) {
	if len(parent.Namespace) != 0 {
		setList := &corev1alpha1.ObjectSetList{}
		if err := Client.List(ctx, setList, client.InNamespace(parent.Namespace)); err != nil {
			return nil, fmt.Errorf("listing ObjectSets: %w", err)
		}
		setPointers := make([]*corev1alpha1.ObjectSet, len(setList.Items))
		for i := range setList.Items {
			setPointers[i] = &setList.Items[i]
		}
		setPointers = filterListByOwningObjectPrefix(parent.Name, setPointers)
		return mapToNamespacedNames(setPointers), nil
	}

	setList := &corev1alpha1.ClusterObjectSetList{}
	if err := Client.List(ctx, setList); err != nil {
		return nil, fmt.Errorf("listing ClusterObjectSets: %w", err)
	}
	setPointers := make([]*corev1alpha1.ClusterObjectSet, len(setList.Items))
	for i := range setList.Items {
		setPointers[i] = &setList.Items[i]
	}
	setPointers = filterListByOwningObjectPrefix(parent.Name, setPointers)
	return mapToNamespacedNames(setPointers), nil
}

func getObjectSliceNamesControlledBy(ctx context.Context, parent types.NamespacedName) ([]types.NamespacedName, error) {
	if len(parent.Namespace) != 0 {
		sliceList := &corev1alpha1.ObjectSliceList{}
		if err := Client.List(ctx, sliceList, client.InNamespace(parent.Namespace)); err != nil {
			return nil, fmt.Errorf("listing ObjectSlices: %w", err)
		}
		slicePointers := make([]*corev1alpha1.ObjectSlice, len(sliceList.Items))
		for i := range sliceList.Items {
			slicePointers[i] = &sliceList.Items[i]
		}
		slicePointers = filterListByOwningObjectPrefix(parent.Name, slicePointers)
		return mapToNamespacedNames(slicePointers), nil
	}

	sliceList := &corev1alpha1.ClusterObjectSliceList{}
	if err := Client.List(ctx, sliceList); err != nil {
		return nil, fmt.Errorf("listing clusterObjectSlices: %w", err)
	}
	slicePointers := make([]*corev1alpha1.ClusterObjectSlice, len(sliceList.Items))
	for i := range sliceList.Items {
		slicePointers[i] = &sliceList.Items[i]
	}
	slicePointers = filterListByOwningObjectPrefix(parent.Name, slicePointers)
	return mapToNamespacedNames(slicePointers), nil
}

func assertAmountOfSliceObjectsControlledPerObjectSet(
	ctx context.Context, t *testing.T, name types.NamespacedName, amount int,
) {
	t.Helper()
	sets, err := getObjectSetNamesControlledBy(ctx, name)
	require.NoError(t, err, "getting (Cluster)ObjectSet names for controller",
		"err", err,
		"controller", name,
	)

	t.Logf("got %d (Cluster)ObjectSet names for controller %s", len(sets), name)
	for _, set := range sets {
		slices, err := getObjectSliceNamesControlledBy(ctx, set)
		require.NoError(t, err, "getting (Cluster)ObjectSlice names controlled by (Cluster)ObjectSet",
			"err", err,
			"controller", set,
		)
		t.Logf("got %d (Cluster)ObjectSlice names for controller %s", len(slices), set)
		assertLenWithJSON(t, slices, amount)
	}
}

// assert len but print json output.
func assertLenWithJSON[T any](t *testing.T, obj []T, l int) {
	t.Helper()
	if len(obj) == l {
		return
	}

	j, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	t.Error(fmt.Sprintf("should be of len %d", l), string(j))
}

func requireDeployPackage(ctx context.Context, t *testing.T, pkg, objectDeployment client.Object) {
	t.Helper()

	require.NoError(t, Client.Create(ctx, pkg))
	cleanupOnSuccess(ctx, t, pkg)

	timeoutOpt := wait.WithTimeout(120 * time.Second)

	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg, corev1alpha1.PackageUnpacked, metav1.ConditionTrue, timeoutOpt))
	// Condition Mapping from Deployment
	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg, "my-prefix/Progressing", metav1.ConditionTrue, timeoutOpt))
	require.NoError(t,
		Waiter.WaitForCondition(ctx, pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue, timeoutOpt))

	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name: pkg.GetName(), Namespace: pkg.GetNamespace(),
	}, objectDeployment))
}

func hashCollisionTestProbe() []corev1alpha1.ObjectSetProbe {
	return []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Kind: "ConfigMap",
				},
			},
			Probes: []corev1alpha1.Probe{
				{
					FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
						FieldA: ".metadata.name",
						FieldB: ".data.name",
					},
				},
			},
		},
	}
}

func cmTemplate(name string, namespace string, data map[string]string, t require.TestingT) unstructured.Unstructured {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"test.package-operator.run/test-1": "True"},
		},
		Data: data,
	}
	GVK, err := apiutil.GVKForObject(&cm, Scheme)
	require.NoError(t, err)
	cm.SetGroupVersionKind(GVK)

	resObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cm)
	require.NoError(t, err)
	return unstructured.Unstructured{Object: resObj}
}

func objectDeploymentTemplate(
	objectSetPhases []corev1alpha1.ObjectSetTemplatePhase,
	probes []corev1alpha1.ObjectSetProbe, name string, revisionHistoryLimit int32,
) *corev1alpha1.ObjectDeployment {
	label := "test.package-operator.run/" + name
	return &corev1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectDeploymentSpec{
			RevisionHistoryLimit: &revisionHistoryLimit,
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{label: "True"},
			},
			Template: corev1alpha1.ObjectSetTemplate{
				Metadata: metav1.ObjectMeta{
					Labels: map[string]string{label: "True"},
				},
				Spec: corev1alpha1.ObjectSetTemplateSpec{
					Phases:             objectSetPhases,
					AvailabilityProbes: probes,
				},
			},
		},
	}
}

func deploymentTemplate(deploymentName string, podImage string, t require.TestingT) unstructured.Unstructured {
	obj := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: map[string]string{"test.package-operator.run/test-1": "True"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.package-operator.run/test-1": "True"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "nginx",
					Labels: map[string]string{"test.package-operator.run/test-1": "True"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: podImage,
						},
					},
				},
			},
		},
	}

	GVK, err := apiutil.GVKForObject(&obj, Scheme)
	require.NoError(t, err)
	obj.SetGroupVersionKind(GVK)
	resObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	require.NoError(t, err)
	return unstructured.Unstructured{Object: resObj}
}

func clusterPackageTemplate(name string) client.Object {
	meta := metav1.ObjectMeta{
		Name: name,
	}
	spec := corev1alpha1.PackageSpec{
		Image: SuccessTestPackageImage,
		Config: &runtime.RawExtension{
			Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
		},
	}

	return &corev1alpha1.ClusterPackage{
		ObjectMeta: meta,
		Spec:       spec,
	}
}

func requireCondition(
	ctx context.Context, t *testing.T, object client.Object,
	conditionType string, conditionStatus metav1.ConditionStatus,
) {
	t.Helper()

	require.NoError(t,
		Waiter.WaitForCondition(ctx,
			object, conditionType, conditionStatus,
			wait.WithTimeout(60*time.Second),
		),
	)
}

func requireClientGet(ctx context.Context, t *testing.T, name, namespace string, object client.Object) {
	t.Helper()

	require.NoError(t,
		Client.Get(ctx,
			client.ObjectKey{
				Name:      name,
				Namespace: namespace,
			},
			object,
		),
	)
}
