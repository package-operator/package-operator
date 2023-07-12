package objectsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestGenericObjectSetPhase(t *testing.T) {
	t.Parallel()

	objectSet := newGenericObjectSetPhase(testScheme).(*GenericObjectSetPhase)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectSetPhase{}, co)

	objectSet.Status.Conditions = []metav1.Condition{
		{},
	}
	assert.Equal(t, objectSet.Status.Conditions, objectSet.GetConditions())

	objectSet.SetPaused(true)
	assert.True(t, objectSet.IsPaused())

	phase := corev1alpha1.ObjectSetTemplatePhase{
		Name:    "tst",
		Class:   "my-class",
		Objects: []corev1alpha1.ObjectSetObject{{}},
	}
	objectSet.SetPhase(phase)
	assert.Equal(t, "my-class",
		objectSet.Labels[corev1alpha1.ObjectSetPhaseClassLabel])
	assert.Equal(t, phase.Objects, objectSet.Spec.Objects)

	objectSet.Status.ControllerOf = []corev1alpha1.ControlledObjectReference{
		{},
	}
	assert.Equal(t, objectSet.Status.ControllerOf, objectSet.GetStatusControllerOf())

	probes := []corev1alpha1.ObjectSetProbe{{}}
	objectSet.SetAvailabilityProbes(probes)
	assert.Equal(t, probes, objectSet.Spec.AvailabilityProbes)

	var revision int64 = 34
	objectSet.SetRevision(revision)
	assert.Equal(t, revision, objectSet.Spec.Revision)

	previous := []corev1alpha1.PreviousRevisionReference{
		{},
	}
	objectSet.SetPrevious(previous)
	assert.Equal(t, previous, objectSet.Spec.Previous)
}

func TestGenericClusterObjectSetPhase(t *testing.T) {
	t.Parallel()
	objectSet := newGenericClusterObjectSetPhase(testScheme).(*GenericClusterObjectSetPhase)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectSetPhase{}, co)

	objectSet.Status.Conditions = []metav1.Condition{
		{},
	}
	assert.Equal(t, objectSet.Status.Conditions, objectSet.GetConditions())

	objectSet.SetPaused(true)
	assert.True(t, objectSet.IsPaused())

	phase := corev1alpha1.ObjectSetTemplatePhase{
		Name:    "tst",
		Class:   "my-class",
		Objects: []corev1alpha1.ObjectSetObject{{}},
	}
	objectSet.SetPhase(phase)
	assert.Equal(t, "my-class",
		objectSet.Labels[corev1alpha1.ObjectSetPhaseClassLabel])
	assert.Equal(t, phase.Objects, objectSet.Spec.Objects)

	objectSet.Status.ControllerOf = []corev1alpha1.ControlledObjectReference{
		{},
	}
	assert.Equal(t, objectSet.Status.ControllerOf, objectSet.GetStatusControllerOf())

	probes := []corev1alpha1.ObjectSetProbe{{}}
	objectSet.SetAvailabilityProbes(probes)
	assert.Equal(t, probes, objectSet.Spec.AvailabilityProbes)

	var revision int64 = 34
	objectSet.SetRevision(revision)
	assert.Equal(t, revision, objectSet.Spec.Revision)

	previous := []corev1alpha1.PreviousRevisionReference{
		{},
	}
	objectSet.SetPrevious(previous)
	assert.Equal(t, previous, objectSet.Spec.Previous)
}
