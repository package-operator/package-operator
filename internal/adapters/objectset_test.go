package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectSet(t *testing.T) {
	t.Parallel()

	objectSet := NewObjectSet(testScheme).(*ObjectSetAdapter)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectSet{}, co)

	objectSet.Status.Conditions = []metav1.Condition{{}}
	assert.Equal(t, objectSet.Status.Conditions, *objectSet.GetStatusConditions())

	assert.False(t, objectSet.IsSpecPaused())
	assert.False(t, objectSet.IsSpecArchived())
	objectSet.SetSpecPaused()
	assert.True(t, objectSet.IsSpecPaused())
	objectSet.SetSpecArchived()
	assert.True(t, objectSet.IsSpecArchived())

	phases := []corev1alpha1.ObjectSetTemplatePhase{{}}
	objectSet.SetSpecPhases(phases)
	assert.Equal(t, phases, objectSet.GetSpecPhases())

	objectSet.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{}}
	assert.Equal(t, objectSet.Spec.AvailabilityProbes,
		objectSet.GetAvailabilityProbes())

	var revision int64 = 34
	objectSet.SetStatusRevision(revision)
	assert.Equal(t, revision, objectSet.GetStatusRevision())
	objectSet.SetSpecRevision(revision)
	assert.Equal(t, revision, objectSet.GetSpecRevision())

	objectSet.Spec.Previous = []corev1alpha1.PreviousRevisionReference{
		{},
	}
	assert.Equal(t, objectSet.Spec.Previous, objectSet.GetSpecPrevious())

	remotes := []corev1alpha1.RemotePhaseReference{{}}
	objectSet.SetStatusRemotePhases(remotes)
	assert.Equal(t, remotes, objectSet.GetStatusRemotePhases())

	controllerOf := []corev1alpha1.ControlledObjectReference{{}}
	objectSet.SetStatusControllerOf(controllerOf)
	assert.Equal(t, controllerOf, objectSet.GetStatusControllerOf())

	templateSpec := corev1alpha1.ObjectSetTemplateSpec{
		SuccessDelaySeconds: 42,
	}
	objectSet.SetSpecTemplateSpec(templateSpec)
	assert.Equal(t, templateSpec, objectSet.GetSpecTemplateSpec())
	assert.Equal(t, templateSpec.SuccessDelaySeconds, objectSet.GetSpecSuccessDelaySeconds())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetPaused,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsStatusPaused())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetAvailable,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsSpecAvailable())

	objectSet.SetSpecPausedByParent()
	assert.True(t, objectSet.GetSpecPausedByParent())
	objectSet.SetSpecActiveByParent()
	assert.False(t, objectSet.GetSpecPausedByParent())
}

func TestClusterObjectSet(t *testing.T) {
	t.Parallel()

	objectSet := NewClusterObjectSet(testScheme).(*ClusterObjectSetAdapter)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectSet{}, co)

	objectSet.Status.Conditions = []metav1.Condition{{}}
	assert.Equal(t, objectSet.Status.Conditions, *objectSet.GetStatusConditions())

	assert.False(t, objectSet.IsSpecPaused())
	assert.False(t, objectSet.IsSpecArchived())
	objectSet.SetSpecPaused()
	assert.True(t, objectSet.IsSpecPaused())
	objectSet.SetSpecArchived()
	assert.True(t, objectSet.IsSpecArchived())

	phases := []corev1alpha1.ObjectSetTemplatePhase{{}}
	objectSet.SetSpecPhases(phases)
	assert.Equal(t, phases, objectSet.GetSpecPhases())

	objectSet.Spec.AvailabilityProbes = []corev1alpha1.ObjectSetProbe{{}}
	assert.Equal(t, objectSet.Spec.AvailabilityProbes,
		objectSet.GetAvailabilityProbes())

	var revision int64 = 34
	objectSet.SetStatusRevision(revision)
	assert.Equal(t, revision, objectSet.GetStatusRevision())
	objectSet.SetSpecRevision(revision)
	assert.Equal(t, revision, objectSet.GetSpecRevision())

	objectSet.Spec.Previous = []corev1alpha1.PreviousRevisionReference{
		{},
	}
	assert.Equal(t, objectSet.Spec.Previous, objectSet.GetSpecPrevious())

	remotes := []corev1alpha1.RemotePhaseReference{{}}
	objectSet.SetStatusRemotePhases(remotes)
	assert.Equal(t, remotes, objectSet.GetStatusRemotePhases())

	controllerOf := []corev1alpha1.ControlledObjectReference{{}}
	objectSet.SetStatusControllerOf(controllerOf)
	assert.Equal(t, controllerOf, objectSet.GetStatusControllerOf())

	templateSpec := corev1alpha1.ObjectSetTemplateSpec{
		SuccessDelaySeconds: 42,
	}
	objectSet.SetSpecTemplateSpec(templateSpec)
	assert.Equal(t, templateSpec, objectSet.GetSpecTemplateSpec())
	assert.Equal(t, templateSpec.SuccessDelaySeconds, objectSet.GetSpecSuccessDelaySeconds())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetPaused,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsStatusPaused())

	objectSet.Status.Conditions = []metav1.Condition{{
		Type:   corev1alpha1.ObjectSetAvailable,
		Status: metav1.ConditionTrue,
	}}
	assert.True(t, objectSet.IsSpecAvailable())

	objectSet.SetSpecPausedByParent()
	assert.True(t, objectSet.GetSpecPausedByParent())
	objectSet.SetSpecActiveByParent()
	assert.False(t, objectSet.GetSpecPausedByParent())
}
