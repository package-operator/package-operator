package objectsets

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/package-operator/internal/testutil/controllersmocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/mock"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
)

type phaseReconcilerMock = controllersmocks.PhaseReconcilerMock

type remotePhaseReconcilerMock struct {
	mock.Mock
}

func (m *remotePhaseReconcilerMock) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Get(0).([]corev1alpha1.ControlledObjectReference),
		args.Get(1).(controllers.ProbingResult),
		args.Error(2)
}

func (m *remotePhaseReconcilerMock) Teardown(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Bool(0), args.Error(1)
}

func TestObjectSetPhasesReconciler_Reconcile(t *testing.T) {
	pr := &phaseReconcilerMock{}
	remotePr := &remotePhaseReconcilerMock{}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{}, nil
	}
	r := newObjectSetPhasesReconciler(testScheme, pr, remotePr, lookup)

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
		Return([]client.Object{}, controllers.ProbingResult{}, nil)
	remotePr.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
		Return([]corev1alpha1.ControlledObjectReference{}, controllers.ProbingResult{}, nil)

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
			r := newObjectSetPhasesReconciler(testScheme, pr, remotePr, lookup)

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
