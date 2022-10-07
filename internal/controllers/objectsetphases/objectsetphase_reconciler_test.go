package objectsetphases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/testutil"
	"package-operator.run/package-operator/internal/testutil/controllersmocks"
	"package-operator.run/package-operator/internal/testutil/ownerhandlingmocks"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

type phaseReconcilerMock = controllersmocks.PhaseReconcilerMock

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
			ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
			r := newObjectSetPhaseReconciler(testScheme, m, lookup, ownerStrategy)

			if test.condition.Reason == "ProbeFailure" {
				m.
					On("ReconcilePhase", mock.Anything, objectSetPhase, objectSetPhase.GetPhase(), mock.Anything, previousList).
					Return([]client.Object{}, controllers.ProbingResult{PhaseName: "this"}, nil).
					Once()
			} else {
				m.
					On("ReconcilePhase", mock.Anything, objectSetPhase, objectSetPhase.GetPhase(), mock.Anything, previousList).
					Return([]client.Object{}, controllers.ProbingResult{}, nil).
					Once()
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
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	m := &phaseReconcilerMock{}
	m.On("TeardownPhase", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	r := newObjectSetPhaseReconciler(testScheme, m, lookup, ownerStrategy)
	_, err := r.Teardown(context.Background(), objectSetPhase)
	assert.NoError(t, err)
	m.AssertCalled(t, "TeardownPhase", mock.Anything, mock.Anything, mock.Anything)
}
