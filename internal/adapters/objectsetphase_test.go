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
	objectSetPhase.SetRevision(revision)
	assert.Equal(t, revision, objectSetPhase.GetRevision())

	paused := true
	objectSetPhase.SetPaused(paused)
	assert.Equal(t, paused, objectSetPhase.IsSpecPaused())

	objectSetPhase.ObjectSetPhase.Status.Conditions = []metav1.Condition{}
	assert.Equal(
		t,
		len(objectSetPhase.ObjectSetPhase.Status.Conditions),
		len(*objectSetPhase.GetConditions()),
	)

	objectSetPhase.SetAvailabilityProbes([]corev1alpha1.ObjectSetProbe{})
	assert.Equal(
		t,
		len(objectSetPhase.ObjectSetPhase.Spec.AvailabilityProbes),
		len(objectSetPhase.GetAvailabilityProbes()),
	)
}

func TestClusterObjectSetPhase(t *testing.T) {
	t.Parallel()

	clusterObjectSetPhase := NewClusterObjectSetPhaseAccessor(testScheme).(*ClusterObjectSetPhaseAdapter)

	co := clusterObjectSetPhase.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectSetPhase{}, co)

	var revision int64 = 2
	clusterObjectSetPhase.SetRevision(revision)
	assert.Equal(t, revision, clusterObjectSetPhase.GetRevision())

	paused := true
	clusterObjectSetPhase.SetPaused(paused)
	assert.Equal(t, paused, clusterObjectSetPhase.IsSpecPaused())

	clusterObjectSetPhase.ClusterObjectSetPhase.Status.Conditions = []metav1.Condition{}
	assert.Equal(
		t,
		len(clusterObjectSetPhase.ClusterObjectSetPhase.Status.Conditions),
		len(*clusterObjectSetPhase.GetConditions()),
	)

	clusterObjectSetPhase.SetAvailabilityProbes([]corev1alpha1.ObjectSetProbe{})
	assert.Equal(
		t,
		len(clusterObjectSetPhase.ClusterObjectSetPhase.Spec.AvailabilityProbes),
		len(clusterObjectSetPhase.GetAvailabilityProbes()),
	)
}
