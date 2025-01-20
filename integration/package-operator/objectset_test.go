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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
)

func TestCollisionPreventionPreventUnowned(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-prevent-unowned-cm",
			Namespace: "default",
		},
		Data: map[string]string{"banana": "bread"},
	}
	require.NoError(t, Client.Create(ctx, existing))
	cleanupOnSuccess(ctx, t, existing)

	obectSetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-collision-prevention-prevent-unowned-cm"},
		Data:       map[string]string{"banana": "bread"},
	}
	cmGVK, err := apiutil.GVKForObject(obectSetCM, Scheme)
	require.NoError(t, err)
	obectSetCM.SetGroupVersionKind(cmGVK)
	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obectSetCM)
	require.NoError(t, err)
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-prevent-unowned",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "Prevent",
								Object:              unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionFalse))
}

func TestCollisionPreventionPreventOwned(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))
	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-prevent-owned-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                "a",
				Kind:               "notus",
				Name:               "notuse",
				APIVersion:         "3",
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}},
		},
		Data: map[string]string{"banana": "bread"},
	}

	require.NoError(t, Client.Create(ctx, existing))
	cleanupOnSuccess(ctx, t, existing)

	objsetSetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-collision-prevention-prevent-owned-cm"},
		Data:       map[string]string{"banana": "bread"},
	}
	cmGVK, err := apiutil.GVKForObject(objsetSetCM, Scheme)
	require.NoError(t, err)
	objsetSetCM.SetGroupVersionKind(cmGVK)
	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objsetSetCM)
	require.NoError(t, err)
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-prevent-owned",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "Prevent",
								Object:              unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionFalse))
}

func TestCollisionPreventionInvalidSet(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	objectSetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-collision-prevention-invalid-set",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                "a",
				Kind:               "notus",
				Name:               "notuse",
				APIVersion:         "3",
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}},
		},
		Data: map[string]string{"banana": "bread"},
	}
	cmGVK, err := apiutil.GVKForObject(objectSetCM, Scheme)
	require.NoError(t, err)
	objectSetCM.SetGroupVersionKind(cmGVK)
	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objectSetCM)
	require.NoError(t, err)
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-invalid-set",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "IfNoController",
								Object:              unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionFalse))
}

func TestCollisionPreventionIfNoControllerOwned(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-if-no-controller-owned-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                "a",
				Kind:               "notus",
				Name:               "notuse",
				APIVersion:         "3",
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}},
		},
		Data: map[string]string{"banana": "bread"},
	}

	require.NoError(t, Client.Create(ctx, existing))
	cleanupOnSuccess(ctx, t, existing)
	objectSetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-collision-prevention-if-no-controller-owned-cm",
		},
		Data: map[string]string{"banana": "bread"},
	}
	cmGVK, err := apiutil.GVKForObject(objectSetCM, Scheme)
	require.NoError(t, err)
	objectSetCM.SetGroupVersionKind(cmGVK)
	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objectSetCM)
	require.NoError(t, err)
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-if-no-controller-owned",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "IfNoController",
								Object:              unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionFalse))
}

func TestCollisionPreventionIfNoControllerUnowned(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	require.NoError(t, Client.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-collision-prevention-if-no-controller-unowned-cm", Namespace: "default"},
		Data:       map[string]string{"banana": "bread"},
	}))

	objectSetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-collision-prevention-if-no-controller-unowned-cm"},
		Data:       map[string]string{"banana": "bread"},
	}
	cmGVK, err := apiutil.GVKForObject(objectSetCM, Scheme)
	require.NoError(t, err)
	objectSetCM.SetGroupVersionKind(cmGVK)
	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objectSetCM)
	require.NoError(t, err)
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-if-no-controller-unowned",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "IfNoController",
								Object:              unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	// expect cm-4 to be reported under "ControllerOf"
	expectedControllerOf := []corev1alpha1.ControlledObjectReference{
		{Kind: "ConfigMap", Name: objectSetCM.Name, Namespace: "default"},
	}
	require.NoError(t, Waiter.WaitForObject(ctx, objectSet,
		"Waiting for .status.controllerOf to be updated",
		func(client.Object) (bool, error) {
			return reflect.DeepEqual(objectSet.Status.ControllerOf, expectedControllerOf), nil
		}),
	)

	// Expect ObjectSet to become Available.
	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))
}

func TestCollisionPreventionNoneUnowned(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	require.NoError(t, Client.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-none-unowned-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				UID:                "a",
				Kind:               "notus",
				Name:               "notuse",
				APIVersion:         "3",
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}},
		},
		Data: map[string]string{"banana": "bread"},
	}))

	objectSetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-collision-prevention-none-unowned-cm"},
		Data:       map[string]string{"banana": "bread"},
	}
	cmGVK, err := apiutil.GVKForObject(objectSetCM, Scheme)
	require.NoError(t, err)
	objectSetCM.SetGroupVersionKind(cmGVK)
	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objectSetCM)
	require.NoError(t, err)
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-none-unowned",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "None",
								Object:              unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	// expect cm-4 to be reported under "ControllerOf"
	expectedControllerOf := []corev1alpha1.ControlledObjectReference{
		{Kind: "ConfigMap", Name: objectSetCM.Name, Namespace: "default"},
	}
	require.NoError(t, Waiter.WaitForObject(ctx, objectSet,
		"Waiting for .status.controllerOf to be updated",
		func(client.Object) (bool, error) {
			return reflect.DeepEqual(objectSet.Status.ControllerOf, expectedControllerOf), nil
		}),
	)

	// Expect ObjectSet to become Available.
	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))
}

func TestCollisionPreventionNoneOwned(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	require.NoError(t, Client.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-collision-prevention-none-owned-cm", Namespace: "default"},
		Data:       map[string]string{"banana": "bread"},
	}))

	objectSetCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-collision-prevention-none-owned-cm"},
		Data:       map[string]string{"banana": "bread"},
	}
	cmGVK, err := apiutil.GVKForObject(objectSetCM, Scheme)
	require.NoError(t, err)
	objectSetCM.SetGroupVersionKind(cmGVK)
	cm4Obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objectSetCM)
	require.NoError(t, err)
	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-collision-prevention-none-owned",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "None",
								Object:              unstructured.Unstructured{Object: cm4Obj},
							},
						},
					},
				},
			},
		},
	}

	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)

	// expect cm-4 to be reported under "ControllerOf"
	expectedControllerOf := []corev1alpha1.ControlledObjectReference{
		{Kind: "ConfigMap", Name: objectSetCM.Name, Namespace: "default"},
	}
	require.NoError(t, Waiter.WaitForObject(ctx, objectSet,
		"Waiting for .status.controllerOf to be updated",
		func(client.Object) (bool, error) {
			return reflect.DeepEqual(objectSet.Status.ControllerOf, expectedControllerOf), nil
		}),
	)

	// Expect ObjectSet to become Available.
	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))
}

// Simple Setup, Pause and Teardown test.
func TestObjectSet_setupPauseTeardown(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetSetupPauseTeardownTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_handover(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetHandoverTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_orphanCascadeDeletion(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetOrphanCascadeDeletionTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_teardownObjectNotControlledAnymore(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-teardown-uncontrolled",
			Namespace: "default",
		},
		Data: map[string]string{
			"banana":       "bread",
			"uncontrolled": "emotions",
		},
	}

	cmGVK, err := apiutil.GVKForObject(configMap, Scheme)
	require.NoError(t, err)
	configMap.SetGroupVersionKind(cmGVK)

	unstructuredCM, err := runtime.DefaultUnstructuredConverter.ToUnstructured(configMap)
	require.NoError(t, err)

	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-teardown-uncontrolled",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "Prevent",
								Object:              unstructured.Unstructured{Object: unstructuredCM},
							},
						},
					},
				},
			},
		},
	}

	// Apply ObjectSet and wait for it to become available.
	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)
	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

	// Fetch ConfigMap from API and disable the controller flag on its owner reference.
	actualConfigMap := &corev1.ConfigMap{}
	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(configMap), actualConfigMap))
	actualConfigMap.OwnerReferences[0].Controller = ptr.To(false)
	require.NoError(t, Client.Update(ctx, actualConfigMap))

	// Delete ObjectSet.
	require.NoError(t, Client.Delete(ctx, objectSet))

	// Wait for owner reference and dynamic cache label on ConfigMap to be removed.
	require.NoError(t,
		Waiter.WaitForObject(
			ctx, actualConfigMap, "internal ownerReference to be removed",
			func(client.Object) (bool, error) {
				configMap := &corev1.ConfigMap{}
				err := Client.Get(ctx, client.ObjectKeyFromObject(actualConfigMap), configMap)
				ownerRefFound := false
				for _, owner := range configMap.GetOwnerReferences() {
					if owner.Name == objectSet.Name {
						ownerRefFound = true
					}
				}
				label := configMap.GetLabels()
				_, labelFound := label[constants.DynamicCacheLabel]
				return !ownerRefFound && !labelFound, err
			}, wait.WithTimeout(40*time.Second),
		))
}
