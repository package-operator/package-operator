package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectSetPhase(t *testing.T) {
	t.Parallel()

	objectSetPhase := NewObjectSetPhaseAccessor(testScheme).(*ObjectSetPhaseAdapter)

	co := objectSetPhase.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectSetPhase{}, co)

	var revision int64 = 2
	objectSetPhase.SetStatusRevision(revision)
	assert.Equal(t, revision, objectSetPhase.GetStatusRevision())

	paused := true
	objectSetPhase.SetSpecPaused(paused)
	assert.Equal(t, paused, objectSetPhase.IsSpecPaused())

	objectSetPhase.Status.Conditions = []metav1.Condition{}
	assert.Len(
		t,
		*objectSetPhase.GetStatusConditions(), len(objectSetPhase.Status.Conditions),
	)

	objectSetPhase.SetAvailabilityProbes([]corev1alpha1.ObjectSetProbe{})
	assert.Len(
		t,
		objectSetPhase.GetAvailabilityProbes(), len(objectSetPhase.Spec.AvailabilityProbes),
	)
}

func TestClusterObjectSetPhase(t *testing.T) {
	t.Parallel()

	clusterObjectSetPhase := NewClusterObjectSetPhaseAccessor(testScheme).(*ClusterObjectSetPhaseAdapter)

	co := clusterObjectSetPhase.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectSetPhase{}, co)

	var revision int64 = 2
	clusterObjectSetPhase.SetStatusRevision(revision)
	assert.Equal(t, revision, clusterObjectSetPhase.GetStatusRevision())

	paused := true
	clusterObjectSetPhase.SetSpecPaused(paused)
	assert.Equal(t, paused, clusterObjectSetPhase.IsSpecPaused())

	clusterObjectSetPhase.Status.Conditions = []metav1.Condition{}
	assert.Len(
		t,
		*clusterObjectSetPhase.GetStatusConditions(), len(clusterObjectSetPhase.Status.Conditions),
	)

	clusterObjectSetPhase.SetAvailabilityProbes([]corev1alpha1.ObjectSetProbe{})
	assert.Len(
		t,
		clusterObjectSetPhase.GetAvailabilityProbes(),
		len(clusterObjectSetPhase.Spec.AvailabilityProbes),
	)
}
