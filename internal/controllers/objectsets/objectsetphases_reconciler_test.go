package objectsets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/package-operator/internal/probing"

	"github.com/stretchr/testify/mock"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
)

type remotePhaseReconcilerMock struct {
	mock.Mock
}

func (m *remotePhaseReconcilerMock) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (err error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Error(0)
}

func (m *remotePhaseReconcilerMock) Teardown(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Bool(0), args.Error(1)
}

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

func TestObjectSetPhasesReconciler_Reconcile(t *testing.T) {
	pr := &phaseReconcilerMock{}
	remotePr := &remotePhaseReconcilerMock{}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{}, nil
	}
	r := newObjectSetPhasesReconciler(pr, remotePr, lookup)

	phase1 := corev1alpha1.ObjectSetTemplatePhase{
		Name: "phase1",
	}
	phase2 := corev1alpha1.ObjectSetTemplatePhase{
		Name:  "phase2",
		Class: "class",
	}

	os := &GenericObjectSet{}
	os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
		phase1,
		phase2,
	}

	pr.On("ReconcilePhase", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	remotePr.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	res, err := r.Reconcile(context.Background(), os)
	assert.Empty(t, res)
	assert.NoError(t, err)

	pr.AssertCalled(t, "ReconcilePhase", mock.Anything, mock.Anything, phase1, mock.Anything, mock.Anything)
	remotePr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything, phase2)

	conds := *os.GetConditions()
	require.Len(t, conds, 2)
	var succeededCond, availableCond metav1.Condition
	for _, cond := range conds {
		if cond.Type == corev1alpha1.ObjectSetSucceeded {
			succeededCond = cond
		} else if cond.Type == corev1alpha1.ObjectSetAvailable {
			availableCond = cond
		}
	}
	assert.Equal(t, metav1.ConditionTrue, succeededCond.Status)
	assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
}

func TestObjectSetPhasesReconciler_Teardown(t *testing.T) {
	tests := []struct {
		name                string
		firstTeardownFinish bool
	}{
		{
			"confirm phase2 torndown first",
			false,
		},
		{
			"all teardowns finish",
			true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pr := &phaseReconcilerMock{}
			remotePr := &remotePhaseReconcilerMock{}
			lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
				return []controllers.PreviousObjectSet{}, nil
			}
			r := newObjectSetPhasesReconciler(pr, remotePr, lookup)

			phase1 := corev1alpha1.ObjectSetTemplatePhase{
				Name: "phase1",
			}
			phase2 := corev1alpha1.ObjectSetTemplatePhase{
				Name:  "phase2",
				Class: "class",
			}

			os := &GenericObjectSet{}
			os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
				phase1,
				phase2,
			}
			remotePr.On("Teardown", mock.Anything, mock.Anything, mock.Anything).
				Return(test.firstTeardownFinish, nil).Once()
			pr.On("TeardownPhase", mock.Anything, mock.Anything, mock.Anything).
				Return(true, nil).Maybe()

			done, err := r.Teardown(context.Background(), os)
			assert.Equal(t, test.firstTeardownFinish, done)
			assert.NoError(t, err)
			remotePr.AssertCalled(t, "Teardown", mock.Anything, mock.Anything, phase2)
			if test.firstTeardownFinish {
				pr.AssertCalled(t, "TeardownPhase", mock.Anything, mock.Anything, phase1)
			}
		})
	}
}
