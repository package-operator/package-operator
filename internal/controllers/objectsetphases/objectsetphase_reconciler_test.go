package objectsetphases

import (
	"context"
	"testing"

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
	args := m.Called(ctx, owner, phase, probe, previous)
	return args.Error(0)
}

func (m *phaseReconcilerMock) TeardownPhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, owner, phase)
	return args.Bool(0), args.Error(1)
}

func TestPhaseReconciler_Reconcile(t *testing.T) {
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	previousObject := newGenericObjectSet(scheme)
	previousObject.ClientObject().SetName("test")
	previousList := []controllers.PreviousObjectSet{previousObject}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return previousList, nil
	}

	tests := []struct {
		name      string
		condition metav1.Condition
	}{
		{
			name: "probe failed",
			condition: metav1.Condition{
				Status: metav1.ConditionFalse,
				Reason: "ProbeFailure",
			},
		},
		{
			name: "probe passed",
			condition: metav1.Condition{
				Status: metav1.ConditionTrue,
				Reason: "Available",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objectSetPhase := newGenericObjectSetPhase(scheme)
			objectSetPhase.ClientObject().SetName("testPhaseOwner")
			m := &phaseReconcilerMock{}
			r := newObjectSetPhaseReconciler(m, lookup)

			if test.condition.Reason == "ProbeFailure" {
				m.On("ReconcilePhase", mock.Anything, objectSetPhase, objectSetPhase.GetPhase(), mock.Anything, previousList).
					Return(&controllers.PhaseProbingFailedError{}).Once()
			} else {
				m.On("ReconcilePhase", mock.Anything, objectSetPhase, objectSetPhase.GetPhase(), mock.Anything, previousList).
					Return(nil).Once()
			}

			res, err := r.Reconcile(context.Background(), objectSetPhase)
			assert.Empty(t, res)
			assert.NoError(t, err)

			conds := *objectSetPhase.GetConditions()
			assert.Len(t, conds, 1)
			cond := conds[0]
			assert.Equal(t, corev1alpha1.ObjectSetPhaseAvailable, cond.Type)
			assert.Equal(t, test.condition.Status, cond.Status)
			assert.Equal(t, test.condition.Reason, cond.Reason)
		})
	}
}

func TestPhaseReconciler_Teardown(t *testing.T) {
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{}, nil
	}
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := newGenericObjectSetPhase(scheme)
	m := &phaseReconcilerMock{}
	m.On("TeardownPhase", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	r := newObjectSetPhaseReconciler(m, lookup)
	r.Teardown(context.Background(), objectSetPhase)
	m.AssertCalled(t, "TeardownPhase", mock.Anything, mock.Anything, mock.Anything)
}
