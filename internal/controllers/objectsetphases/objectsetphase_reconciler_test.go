package objectsetphases

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/package-operator/internal/probing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/testutil"
)

type phaseReconcilerMock struct {
	mock.Mock
}

func (m *phaseReconcilerMock) ReconcilePhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []controllers.PreviousObjectSet,
) error {
	m.Called(ctx, owner, phase, probe, previous)
	return nil
}

func (m *phaseReconcilerMock) TeardownPhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	m.Called(ctx, owner, phase)
	return false, nil
}

func TestPhaseReconciler_Reconcile(t *testing.T) {
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	prev := newGenericObjectSet(scheme)
	prev.ClientObject().SetName("test")
	prevList := []controllers.PreviousObjectSet{prev}
	prevLookupFunc := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return prevList, nil
	}

	objectSetPhase := newGenericObjectSetPhase(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")

	m := &phaseReconcilerMock{}

	r := newObjectSetPhaseReconciler(m, prevLookupFunc)

	// The first call to ReconcilePhase throws PhaseProbingFailedError
	m.On("ReconcilePhase", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&controllers.PhaseProbingFailedError{}).Once()

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	assert.NoError(t, err)

	prevTest := mock.MatchedBy(func(p []controllers.PreviousObjectSet) bool {
		return reflect.DeepEqual(p, []controllers.PreviousObjectSet{prev})
	})

	m.AssertCalled(t, "ReconcilePhase", mock.Anything, objectSetPhase, objectSetPhase.GetPhase(), mock.Anything, prevTest)
	// Since we mocked the probe failing, the Availability conditions should be false
	cond := (*objectSetPhase.GetConditions())[0]
	expectedCond := metav1.Condition{
		Type:   corev1alpha1.ObjectSetAvailable,
		Status: metav1.ConditionFalse,
		Reason: "ProbeFailure",
	}
	assert.True(t, !equality.Semantic.DeepEqual(cond, expectedCond))

	// The second call to ReconcilePhase does not throw and error
	m.On("ReconcilePhase", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	res, err = r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	assert.NoError(t, err)

	// Since we mocked the probes running successfully, the Availability conditions should be false
	m.AssertCalled(t, "ReconcilePhase", mock.Anything, objectSetPhase, objectSetPhase.GetPhase(), mock.Anything, prevTest)
	fmt.Println(objectSetPhase.GetConditions())
	cond = (*objectSetPhase.GetConditions())[0]
	expectedCond = metav1.Condition{
		Type:   corev1alpha1.ObjectSetPhaseAvailable,
		Status: metav1.ConditionTrue,
		Reason: "Available",
	}
	assert.True(t, !equality.Semantic.DeepEqual(cond, expectedCond))
}
