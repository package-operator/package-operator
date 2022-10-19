package integration

//import (
//	"context"
//	"testing"
//	"time"
//
//	"github.com/go-logr/logr"
//	"github.com/go-logr/logr/testr"
//	"github.com/mt-sre/devkube/dev"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//	corev1 "k8s.io/api/core/v1"
//	"k8s.io/apimachinery/pkg/api/meta"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
//	"k8s.io/apimachinery/pkg/runtime"
//	"k8s.io/apimachinery/pkg/types"
//	"k8s.io/apimachinery/pkg/util/wait"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
//
//	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
//)
//
//// Simple Setup and Pause test.
//func TestObjectSetPhase_setupPause(t *testing.T) {
//	cm := &corev1.ConfigMap{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:   "cm-4",
//			Labels: map[string]string{"test.package-operator.run/test": "True"},
//		},
//		Data: map[string]string{
//			"banana": "bread",
//		},
//	}
//	cmGVK, err := apiutil.GVKForObject(cm, Scheme)
//	require.NoError(t, err)
//	cm.SetGroupVersionKind(cmGVK) // TODO: IDK what this does
//	unstructCm, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
//	require.NoError(t, err)
//	phase := &corev1alpha1.ObjectSetPhase{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      "test-setup-teardown",
//			Namespace: "default",
//		},
//		Spec: corev1alpha1.ObjectSetPhaseSpec{
//			Revision: 1, // No previous
//			AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
//				{
//					Selector: corev1alpha1.ProbeSelector{
//						Kind: &corev1alpha1.PackageProbeKindSpec{
//							Kind: "ConfigMap",
//						},
//						Selector: &metav1.LabelSelector{
//							MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
//						},
//					},
//					Probes: []corev1alpha1.Probe{
//						{
//							FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
//								FieldA: ".metadata.name",
//								FieldB: ".metadata.annotations.name",
//							},
//						},
//					},
//				},
//			},
//			ObjectSetTemplatePhase: corev1alpha1.ObjectSetTemplatePhase{
//				Name:  "phase-1",
//				Class: "hosted-cluster",
//				Objects: []corev1alpha1.ObjectSetObject{
//					{
//						Object: unstructured.Unstructured{Object: unstructCm},
//					},
//				},
//			},
//		},
//	}
//	ctx := logr.NewContext(context.Background(), testr.New(t))
//	require.NoError(t, Client.Create(ctx, phase))
//	cleanupOnSuccess(ctx, t, phase)
//
//	// ------------------------
//	// Test phased setup logic.
//	// ------------------------
//
//	// Wait for false status to be reported.
//	// Phase-1 is expected to fail because of the availabilityProbe.
//	require.NoError(t,
//		Waiter.WaitForCondition(ctx, phase, corev1alpha1.ObjectSetPhaseAvailable, metav1.ConditionFalse))
//	availableCond := meta.FindStatusCondition(phase.Status.Conditions, corev1alpha1.ObjectSetAvailable)
//	require.NotNil(t, availableCond, "Available condition is expected to be reported")
//	assert.Equal(t, "ProbeFailure", availableCond.Reason)
//
//	// expect cm to be reported under "ControllerOf"
//	require.Equal(t, []corev1alpha1.ControlledObjectReference{
//		{
//			Kind:      cm.Kind,
//			Name:      cm.Name,
//			Namespace: cm.Namespace,
//		},
//	}, phase.Status.ControllerOf)
//
//	// expect cm to be present.
//	cmKey := client.ObjectKey{Name: cm.Name, Namespace: cm.Namespace}
//	currentCm := &corev1.ConfigMap{}
//	require.NoError(t, Client.Get(ctx, cmKey, currentCm))
//
//	// Patch cm to pass probe.
//	// -------------------------
//	require.NoError(t,
//		Client.Patch(ctx, currentCm,
//			client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"annotations":{"name":"cm"}}}`))))
//
//	// Expect phase to become Available.
//	require.NoError(t,
//		Waiter.WaitForCondition(ctx, phase, corev1alpha1.ObjectSetPhaseAvailable, metav1.ConditionTrue))
//
//	// expect cm still reported under "ControllerOf" TODO: maybe remove
//	require.Equal(t, []corev1alpha1.ControlledObjectReference{
//		{
//			Kind:      cm.Kind,
//			Name:      cm.Name,
//			Namespace: cm.Namespace,
//		},
//	}, phase.Status.ControllerOf)
//
//	// -----------
//	// Test pause.
//	// -----------
//
//	// Pause ObjectSet.
//	require.NoError(t, Client.Patch(ctx, phase,
//		client.RawPatch(types.MergePatchType, []byte(`{"spec":{"paused":"true"}}`)))) // TODO: Maybe didn't do bool right.
//	// TODO Also maybe have to wait for it to reconcile? There is no status condition for paused ObjectSetPhases
//
//	// should be reconciled to "banana":"bread", if reconciler were not paused.
//	require.NoError(t, Client.Patch(ctx, currentCm,
//		client.RawPatch(types.MergePatchType, []byte(`{"data":{"banana":"toast"}}`))))
//
//	// Wait 5s for the object to be reconciled, which should not happen, because it's paused.
//	require.EqualError(t,
//		Waiter.WaitForObject(ctx, currentCm, "to NOT be reconciled to its desired state", func(obj client.Object) (done bool, err error) {
//			cm := obj.(*corev1.ConfigMap)
//			return cm.Data["banana"] == "bread", nil
//		}, dev.WithTimeout(5*time.Second)), wait.ErrWaitTimeout.Error())
//
//	// Unpause ObjectSet.
//	require.NoError(t, Client.Patch(ctx, phase,
//		client.RawPatch(types.MergePatchType, []byte(`{"spec":{"paused":"false"}}`))))
//
//	// Wait 10s for the object to be reconciled, which should now happen!
//	require.NoError(t,
//		Waiter.WaitForObject(ctx, currentCm, "to be reconciled to its desired state", func(obj client.Object) (done bool, err error) {
//			cm := obj.(*corev1.ConfigMap)
//			return cm.Data["banana"] == "bread", nil
//		}, dev.WithTimeout(10*time.Second)))
//}
