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
	"pkg.package-operator.run/boxcutter/machinery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/boxcuttermocks"
	"package-operator.run/internal/testutil/managedcachemocks"
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
			accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
			accessor := &managedcachemocks.AccessorMock{}
			phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
			phaseEngine := &boxcuttermocks.PhaseEngineMock{}
			phaseResult := &boxcuttermocks.PhaseResultMock{}
			ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
			r := newObjectSetPhaseReconciler(testScheme, accessManager, phaseEngineFactory, lookup, ownerStrategy)
			accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(accessor, nil)

			phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)
			phaseEngine.
				On("Reconcile", mock.Anything, objectSetPhase.ClientObject(),
					objectSetPhase.GetStatusRevision(), mock.Anything, mock.Anything).
				Return(phaseResult, nil).
				Once()
			phaseResult.On("GetObjects").Return([]machinery.ObjectResult{})

			if test.condition.Reason == "ProbeFailure" {
				phaseResult.On("IsComplete").Return(false)
				phaseResult.On("GetProbesStatus").Return("object not ready")
			} else {
				phaseResult.On("IsComplete").Return(true)
				phaseResult.On("GetProbesStatus").Return("")
			}

			res, err := r.Reconcile(context.Background(), objectSetPhase)
			assert.Empty(t, res)
			require.NoError(t, err)

			conds := *objectSetPhase.GetStatusConditions()
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
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	phaseEngine := &boxcuttermocks.PhaseEngineMock{}
	phaseResult := &boxcuttermocks.PhaseResultMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)
	phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)
	phaseEngine.
		On("Reconcile", mock.Anything, objectSetPhase.ClientObject(),
			objectSetPhase.GetStatusRevision(), mock.Anything, mock.Anything).
		Return(phaseResult, controllers.NewExternalResourceNotFoundError(nil)).
		Once()
	phaseResult.On("IsComplete").Return(false)
	phaseResult.On("GetProbesStatus").Return("")

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	require.NoError(t, err)

	assert.Equal(t, reconcile.Result{
		RequeueAfter: controllers.DefaultInitialBackoff,
	}, res)
}

func TestPhaseReconciler_Teardown(t *testing.T) {
	t.Parallel()

	for _, teardownDone := range []bool{true, false} {
		name := "NotDone"
		if teardownDone {
			name = "Done"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
				return []controllers.PreviousObjectSet{}, nil
			}
			scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
			objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
			ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
			accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
			accessor := &managedcachemocks.AccessorMock{}
			phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
			phaseEngine := &boxcuttermocks.PhaseEngineMock{}
			phaseTeardownResult := &boxcuttermocks.PhaseTeardownResultMock{}
			r := newObjectSetPhaseReconciler(testScheme, accessManager, phaseEngineFactory, lookup, ownerStrategy)

			accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(accessor, nil)
			accessManager.On("FreeWithUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)

			phaseEngine.
				On("Teardown", mock.Anything, objectSetPhase.ClientObject(),
					objectSetPhase.GetStatusRevision(), mock.Anything, mock.Anything).
				Return(phaseTeardownResult, nil)

			if teardownDone {
				phaseTeardownResult.On("IsComplete").Return(true)
			} else {
				phaseTeardownResult.On("IsComplete").Return(false)
			}

			_, err := r.Teardown(context.Background(), objectSetPhase)
			require.NoError(t, err)

			phaseEngine.AssertCalled(t, "Teardown", mock.Anything, objectSetPhase.ClientObject(),
				objectSetPhase.GetStatusRevision(), mock.Anything, mock.Anything)

			if teardownDone {
				accessManager.AssertCalled(t, "FreeWithUser", mock.Anything, mock.Anything, mock.Anything)
			} else {
				accessManager.AssertNotCalled(t, "FreeWithUser", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}
