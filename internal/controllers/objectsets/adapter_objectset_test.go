package objectsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var tests = []struct {
	name                string
	startConditions     []metav1.Condition
	expectedStatusPhase corev1alpha1.ObjectSetStatusPhase
}{
	{
		name: "archived true",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetArchived,
				Status: metav1.ConditionTrue,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseArchived,
	},
	{
		name: "archived false",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetArchived,
				Status: metav1.ConditionFalse,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseNotReady,
	},
	{
		name: "paused true",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetPaused,
				Status: metav1.ConditionTrue,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhasePaused,
	},
	{
		name: "available true",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetAvailable,
				Status: metav1.ConditionTrue,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseAvailable,
	},
	{
		name:                "no conditions",
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseNotReady,
	},
}

func TestGenericObjectSet_UpdateStatusPhase(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clusterObjectSet := GenericObjectSet{}
			clusterObjectSet.Status.Conditions = test.startConditions
			clusterObjectSet.UpdateStatusPhase()
			assert.Equal(t, test.expectedStatusPhase, clusterObjectSet.Status.Phase)
		})
	}
}

func TestGenericClusterObjectSet_UpdateStatusPhase(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clusterObjectSet := GenericClusterObjectSet{}
			clusterObjectSet.Status.Conditions = test.startConditions
			clusterObjectSet.UpdateStatusPhase()
			assert.Equal(t, test.expectedStatusPhase, clusterObjectSet.Status.Phase)
		})
	}
}

func TestGenericObjectSet(t *testing.T) {
	objectSet := newGenericObjectSet(testScheme).(*GenericObjectSet)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectSet{}, co)

	objectSet.Status.Conditions = []metav1.Condition{{}}
	assert.Equal(t, objectSet.Status.Conditions, *objectSet.GetConditions())

	assert.False(t, objectSet.IsPaused())
	assert.False(t, objectSet.IsArchived())
	objectSet.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
	assert.True(t, objectSet.IsPaused())
	objectSet.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
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
	assert.Equal(t, controllerOf, objectSet.Status.ControllerOf)
}

func TestGenericClusterObjectSet(t *testing.T) {
	objectSet := newGenericClusterObjectSet(testScheme).(*GenericClusterObjectSet)

	co := objectSet.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectSet{}, co)

	objectSet.Status.Conditions = []metav1.Condition{{}}
	assert.Equal(t, objectSet.Status.Conditions, *objectSet.GetConditions())

	assert.False(t, objectSet.IsPaused())
	assert.False(t, objectSet.IsArchived())
	objectSet.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
	assert.True(t, objectSet.IsPaused())
	objectSet.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
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
	assert.Equal(t, controllerOf, objectSet.Status.ControllerOf)
}
