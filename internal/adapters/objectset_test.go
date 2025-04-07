package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectSet(t *testing.T) {
	t.Parallel()

	objectSet := NewObjectSet(testScheme).(*ObjectSet)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectSet{}, co)

	objectSet.Status.Conditions = []metav1.Condition{{}}
	assert.Equal(t, objectSet.Status.Conditions, *objectSet.GetConditions())

	assert.False(t, objectSet.IsSpecPaused())
	assert.False(t, objectSet.IsArchived())
	objectSet.SetPaused()
	assert.True(t, objectSet.IsSpecPaused())
	objectSet.SetArchived()
	assert.True(t, objectSet.IsArchived())

	phases := []corev1alpha1.ObjectSetTemplatePhase{{}}
	objectSet.SetPhases(phases)
	assert.Equal(t, phases, objectSet.GetPhases())

	objectSet.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{}}
	assert.Equal(t, objectSet.Spec.AvailabilityProbes,
		objectSet.GetAvailabilityProbes())

	var revision int64 = 34
	objectSet.SetRevision(revision)
	assert.Equal(t, revision, objectSet.GetRevision())

	objectSet.Spec.Previous = []corev1alpha1.PreviousRevisionReference{
		{},
	}
	assert.Equal(t, objectSet.Spec.Previous, objectSet.GetPrevious())

	remotes := []corev1alpha1.RemotePhaseReference{{}}
	objectSet.SetRemotePhases(remotes)
	assert.Equal(t, remotes, objectSet.GetRemotePhases())

	controllerOf := []corev1alpha1.ControlledObjectReference{{}}
	objectSet.SetStatusControllerOf(controllerOf)
	assert.Equal(t, controllerOf, objectSet.GetStatusControllerOf())

	templateSpec := corev1alpha1.ObjectSetTemplateSpec{
		SuccessDelaySeconds: 42,
	}
	objectSet.SetTemplateSpec(templateSpec)
	assert.Equal(t, templateSpec, objectSet.GetTemplateSpec())
	assert.Equal(t, templateSpec.SuccessDelaySeconds, objectSet.GetSuccessDelaySeconds())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetPaused,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsStatusPaused())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetAvailable,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsAvailable())

	objectSet.SetPausedByParent()
	assert.True(t, objectSet.GetPausedByParent())
	objectSet.SetActiveByParent()
	assert.False(t, objectSet.GetPausedByParent())
}

func TestClusterObjectSet(t *testing.T) {
	t.Parallel()

	objectSet := NewClusterObjectSet(testScheme).(*ClusterObjectSet)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectSet{}, co)

	objectSet.Status.Conditions = []metav1.Condition{{}}
	assert.Equal(t, objectSet.Status.Conditions, *objectSet.GetConditions())

	assert.False(t, objectSet.IsSpecPaused())
	assert.False(t, objectSet.IsArchived())
	objectSet.SetPaused()
	assert.True(t, objectSet.IsSpecPaused())
	objectSet.SetArchived()
	assert.True(t, objectSet.IsArchived())

	phases := []corev1alpha1.ObjectSetTemplatePhase{{}}
	objectSet.SetPhases(phases)
	assert.Equal(t, phases, objectSet.GetPhases())

	objectSet.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{}}
	assert.Equal(t, objectSet.Spec.AvailabilityProbes,
		objectSet.GetAvailabilityProbes())

	var revision int64 = 34
	objectSet.SetRevision(revision)
	assert.Equal(t, revision, objectSet.GetRevision())

	objectSet.Spec.Previous = []corev1alpha1.PreviousRevisionReference{
		{},
	}
	assert.Equal(t, objectSet.Spec.Previous, objectSet.GetPrevious())

	remotes := []corev1alpha1.RemotePhaseReference{{}}
	objectSet.SetRemotePhases(remotes)
	assert.Equal(t, remotes, objectSet.GetRemotePhases())

	controllerOf := []corev1alpha1.ControlledObjectReference{{}}
	objectSet.SetStatusControllerOf(controllerOf)
	assert.Equal(t, controllerOf, objectSet.GetStatusControllerOf())

	templateSpec := corev1alpha1.ObjectSetTemplateSpec{
		SuccessDelaySeconds: 42,
	}
	objectSet.SetTemplateSpec(templateSpec)
	assert.Equal(t, templateSpec, objectSet.GetTemplateSpec())
	assert.Equal(t, templateSpec.SuccessDelaySeconds, objectSet.GetSuccessDelaySeconds())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetPaused,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsStatusPaused())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetAvailable,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsAvailable())

	objectSet.SetPausedByParent()
	assert.True(t, objectSet.GetPausedByParent())
	objectSet.SetActiveByParent()
	assert.False(t, objectSet.GetPausedByParent())
}
