package objectsetphases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/controllersmocks"
	"package-operator.run/internal/testutil/ownerhandlingmocks"
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
	t.Parallel()
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	previousObject := adapters.NewObjectSet(scheme)
	previousObject.ClientObject().SetName("test")
	previousList := []controllers.PreviousObjectSet{previousObject}
	lookup := func(
		_ context.Context, _ controllers.PreviousOwner,
	) (
		[]controllers.PreviousObjectSet, error, //nolint: unparam
	) {
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
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
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
			require.NoError(t, err)

			conds := *objectSetPhase.GetConditions()
			assert.Len(t, conds, 1)
			cond := conds[0]
			assert.Equal(t, corev1alpha1.ObjectSetPhaseAvailable, cond.Type)
			assert.Equal(t, test.condition.Status, cond.Status)
			assert.Equal(t, test.condition.Reason, cond.Reason)
		})
	}
}

func TestPhaseReconciler_ReconcileBackoff(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	previousObject := adapters.NewObjectSet(scheme)
	previousObject.ClientObject().SetName("test")
	previousList := []controllers.PreviousObjectSet{previousObject}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return previousList, nil
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")
	m := &phaseReconcilerMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	r := newObjectSetPhaseReconciler(testScheme, m, lookup, ownerStrategy)

	m.
		On("ReconcilePhase", mock.Anything, objectSetPhase, objectSetPhase.GetPhase(), mock.Anything, previousList).
		Return([]client.Object{}, controllers.ProbingResult{}, controllers.NewExternalResourceNotFoundError(nil)).
		Once()

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	require.NoError(t, err)

	assert.Equal(t, reconcile.Result{
		RequeueAfter: controllers.DefaultInitialBackoff,
	}, res)
}

func TestPhaseReconciler_Teardown(t *testing.T) {
	t.Parallel()

	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{}, nil
	}
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	m := &phaseReconcilerMock{}
	m.On("TeardownPhase", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	r := newObjectSetPhaseReconciler(testScheme, m, lookup, ownerStrategy)
	_, err := r.Teardown(context.Background(), objectSetPhase)
	require.NoError(t, err)
	m.AssertCalled(t, "TeardownPhase", mock.Anything, mock.Anything, mock.Anything)
}
