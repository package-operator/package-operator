package objectsetphases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"pkg.package-operator.run/boxcutter/machinery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	previousList := []client.Object{previousObject.ClientObject()}
	lookup := func(
		_ context.Context, _ controllers.PreviousOwner,
	) (
		[]client.Object, error, //nolint: unparam
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
			uncachedClient := testutil.NewClient()
			phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
			phaseEngine := &boxcuttermocks.PhaseEngineMock{}
			phaseResult := &boxcuttermocks.PhaseResultMock{}
			ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
			// TODO mock client
			r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
				phaseEngineFactory, lookup, ownerStrategy)
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
				phaseResult.On("String").Return("object not ready")
			} else {
				phaseResult.On("IsComplete").Return(true)
				phaseResult.On("String").Return("")
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
	previousList := []client.Object{previousObject.ClientObject()}
	lookup := func(
		_ context.Context, _ controllers.PreviousOwner,
	) (
		[]client.Object, error,
	) {
		return previousList, nil
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	phaseEngine := &boxcuttermocks.PhaseEngineMock{}
	phaseResult := &boxcuttermocks.PhaseResultMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

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

			lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
				return []client.Object{}, nil
			}
			scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
			objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
			ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
			accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
			uncachedClient := testutil.NewClient()
			accessor := &managedcachemocks.AccessorMock{}
			phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
			phaseEngine := &boxcuttermocks.PhaseEngineMock{}
			phaseTeardownResult := &boxcuttermocks.PhaseTeardownResultMock{}
			r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
				phaseEngineFactory, lookup, ownerStrategy)

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

func TestPhaseReconciler_Teardown_OrphanFinalizer(t *testing.T) {
	t.Parallel()

	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)

	// Add orphan finalizer
	controllerutil.AddFinalizer(objectSetPhase.ClientObject(), "orphan")

	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	cleanupDone, err := r.Teardown(context.Background(), objectSetPhase)
	require.NoError(t, err)
	assert.True(t, cleanupDone, "cleanup should be done when orphan finalizer is present")

	// Verify that no engine methods were called
	accessManager.AssertNotCalled(t, "GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	phaseEngineFactory.AssertNotCalled(t, "New", mock.Anything)
}

func TestPhaseReconciler_Teardown_AccessManagerError(t *testing.T) {
	t.Parallel()

	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	expectedErr := assert.AnError
	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*managedcachemocks.AccessorMock)(nil), expectedErr)

	cleanupDone, err := r.Teardown(context.Background(), objectSetPhase)
	require.Error(t, err)
	assert.False(t, cleanupDone)
	assert.ErrorContains(t, err, "preparing cache")
}

func TestPhaseReconciler_Teardown_PhaseEngineFactoryError(t *testing.T) {
	t.Parallel()

	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)

	expectedErr := assert.AnError
	phaseEngineFactory.On("New", accessor).Return((*boxcuttermocks.PhaseEngineMock)(nil), expectedErr)

	cleanupDone, err := r.Teardown(context.Background(), objectSetPhase)
	require.Error(t, err)
	assert.False(t, cleanupDone)
	assert.Equal(t, expectedErr, err)
}

func TestPhaseReconciler_Teardown_PhaseEngineError(t *testing.T) {
	t.Parallel()

	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	phaseEngine := &boxcuttermocks.PhaseEngineMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)
	phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)

	expectedErr := assert.AnError
	phaseEngine.On("Teardown", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*boxcuttermocks.PhaseTeardownResultMock)(nil), expectedErr)

	cleanupDone, err := r.Teardown(context.Background(), objectSetPhase)
	require.Error(t, err)
	assert.False(t, cleanupDone)
	assert.Equal(t, expectedErr, err)
}

func TestPhaseReconciler_Teardown_FreeWithUserError(t *testing.T) {
	t.Parallel()

	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	phaseEngine := &boxcuttermocks.PhaseEngineMock{}
	phaseTeardownResult := &boxcuttermocks.PhaseTeardownResultMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)
	phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)
	phaseEngine.On("Teardown", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(phaseTeardownResult, nil)
	phaseTeardownResult.On("IsComplete").Return(true)

	expectedErr := assert.AnError
	accessManager.On("FreeWithUser", mock.Anything, mock.Anything, mock.Anything).Return(expectedErr)

	cleanupDone, err := r.Teardown(context.Background(), objectSetPhase)
	require.Error(t, err)
	assert.False(t, cleanupDone)
	assert.ErrorContains(t, err, "freewithuser")
}

func TestPhaseReconciler_Reconcile_LookupPreviousRevisionsError(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	expectedErr := assert.AnError
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return nil, expectedErr
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	require.Error(t, err)
	assert.ErrorContains(t, err, "lookup previous revisions")
}

func TestPhaseReconciler_Reconcile_AccessManagerError(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	expectedErr := assert.AnError
	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*managedcachemocks.AccessorMock)(nil), expectedErr)

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	require.Error(t, err)
	assert.ErrorContains(t, err, "preparing cache")
}

func TestPhaseReconciler_Reconcile_PhaseEngineFactoryError(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)

	expectedErr := assert.AnError
	phaseEngineFactory.On("New", accessor).Return((*boxcuttermocks.PhaseEngineMock)(nil), expectedErr)

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestPhaseReconciler_Reconcile_GenericError(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")
	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	phaseEngine := &boxcuttermocks.PhaseEngineMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}
	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)
	phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)

	expectedErr := assert.AnError
	phaseEngine.On("Reconcile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*boxcuttermocks.PhaseResultMock)(nil), expectedErr)

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestPhaseReconciler_Reconcile_WithObjects(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	previousObject := adapters.NewObjectSet(scheme)
	previousObject.ClientObject().SetName("test")
	previousList := []client.Object{previousObject.ClientObject()}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return previousList, nil
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")

	// Add an object to the phase
	objectSetPhase.SetPhase(corev1alpha1.ObjectSetTemplatePhase{
		Name: "test-phase",
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm",
							"namespace": "default",
						},
					},
				},
				CollisionProtection: corev1alpha1.CollisionProtectionPrevent,
			},
		},
	})

	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	phaseEngine := &boxcuttermocks.PhaseEngineMock{}
	phaseResult := &boxcuttermocks.PhaseResultMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}

	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)
	phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)
	phaseEngine.
		On("Reconcile", mock.Anything, objectSetPhase.ClientObject(),
			objectSetPhase.GetStatusRevision(), mock.Anything, mock.Anything).
		Return(phaseResult, nil).
		Once()
	phaseResult.On("GetObjects").Return([]machinery.ObjectResult{})
	phaseResult.On("IsComplete").Return(true)
	phaseResult.On("String").Return("")

	ownerStrategy.On("GetControllerOf", mock.Anything, mock.Anything).Return([]client.Object{})

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	require.NoError(t, err)

	conds := *objectSetPhase.GetStatusConditions()
	assert.Len(t, conds, 1)
	cond := conds[0]
	assert.Equal(t, corev1alpha1.ObjectSetPhaseAvailable, cond.Type)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, "Available", cond.Reason)
}

func TestPhaseReconciler_Reconcile_PausedState(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]client.Object, error) {
		return []client.Object{}, nil
	}

	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)
	objectSetPhase.ClientObject().SetName("testPhaseOwner")

	// Set paused state
	objectSetPhase.SetSpecPaused(true)

	// Add an object to the phase
	objectSetPhase.SetPhase(corev1alpha1.ObjectSetTemplatePhase{
		Name: "test-phase",
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm",
							"namespace": "default",
						},
					},
				},
				CollisionProtection: corev1alpha1.CollisionProtectionPrevent,
			},
		},
	})

	accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
	accessor := &managedcachemocks.AccessorMock{}
	uncachedClient := testutil.NewClient()
	phaseEngineFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
	phaseEngine := &boxcuttermocks.PhaseEngineMock{}
	phaseResult := &boxcuttermocks.PhaseResultMock{}
	ownerStrategy := &ownerhandlingmocks.OwnerStrategyMock{}

	r := newObjectSetPhaseReconciler(testScheme, accessManager, uncachedClient,
		phaseEngineFactory, lookup, ownerStrategy)

	accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(accessor, nil)
	phaseEngineFactory.On("New", accessor).Return(phaseEngine, nil)

	// The reconciler should pass the WithPaused option to the phase engine
	phaseEngine.
		On("Reconcile", mock.Anything, objectSetPhase.ClientObject(),
			objectSetPhase.GetStatusRevision(), mock.Anything, mock.Anything).
		Return(phaseResult, nil).
		Once()
	phaseResult.On("GetObjects").Return([]machinery.ObjectResult{})
	phaseResult.On("IsComplete").Return(true)
	phaseResult.On("String").Return("")
	ownerStrategy.On("GetControllerOf", mock.Anything, mock.Anything).Return([]client.Object{})

	res, err := r.Reconcile(context.Background(), objectSetPhase)
	assert.Empty(t, res)
	require.NoError(t, err)
}

func TestMapConditions_Success(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)

	// Setup phase with condition mapping
	objectSetPhase.SetPhase(corev1alpha1.ObjectSetTemplatePhase{
		Name: "test-phase",
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm",
							"namespace": "default",
						},
					},
				},
				ConditionMappings: []corev1alpha1.ConditionMapping{
					{
						SourceType:      "Ready",
						DestinationType: "example.com/CustomReady",
					},
				},
			},
		},
	})

	// Create an actual object with conditions
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "test-cm",
				"namespace":  "default",
				"generation": int64(1),
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":               "Ready",
						"status":             "True",
						"reason":             "AllGood",
						"message":            "Everything is working",
						"observedGeneration": int64(1),
					},
				},
			},
		},
	}

	actualObjects := []machinery.Object{obj}
	err := mapConditions(actualObjects, objectSetPhase)
	require.NoError(t, err)

	conds := *objectSetPhase.GetStatusConditions()
	assert.Len(t, conds, 1)
	assert.Equal(t, "example.com/CustomReady", conds[0].Type)
	assert.Equal(t, metav1.ConditionTrue, conds[0].Status)
	assert.Equal(t, "AllGood", conds[0].Reason)
	assert.Equal(t, "Everything is working", conds[0].Message)
}

func TestMapConditions_OutdatedCondition(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)

	// Setup phase with condition mapping
	objectSetPhase.SetPhase(corev1alpha1.ObjectSetTemplatePhase{
		Name: "test-phase",
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm",
							"namespace": "default",
						},
					},
				},
				ConditionMappings: []corev1alpha1.ConditionMapping{
					{
						SourceType:      "Ready",
						DestinationType: "example.com/CustomReady",
					},
				},
			},
		},
	})

	// Create an object with outdated condition (observedGeneration doesn't match)
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "test-cm",
				"namespace":  "default",
				"generation": int64(5),
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":               "Ready",
						"status":             "True",
						"reason":             "AllGood",
						"message":            "Everything is working",
						"observedGeneration": int64(2), // Outdated
					},
				},
			},
		},
	}

	actualObjects := []machinery.Object{obj}
	err := mapConditions(actualObjects, objectSetPhase)
	require.NoError(t, err)

	// Should not map outdated conditions
	conds := *objectSetPhase.GetStatusConditions()
	assert.Empty(t, conds)
}

func TestMapConditions_UnmappedCondition(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)

	// Setup phase with no condition mappings
	objectSetPhase.SetPhase(corev1alpha1.ObjectSetTemplatePhase{
		Name: "test-phase",
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm",
							"namespace": "default",
						},
					},
				},
			},
		},
	})

	// Create an object with conditions but no mappings
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "test-cm",
				"namespace":  "default",
				"generation": int64(1),
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":               "Ready",
						"status":             "True",
						"reason":             "AllGood",
						"message":            "Everything is working",
						"observedGeneration": int64(1),
					},
				},
			},
		},
	}

	actualObjects := []machinery.Object{obj}
	err := mapConditions(actualObjects, objectSetPhase)
	require.NoError(t, err)

	// Should not map unmapped conditions
	conds := *objectSetPhase.GetStatusConditions()
	assert.Empty(t, conds)
}

func TestMapConditions_NoConditions(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)

	// Setup phase
	objectSetPhase.SetPhase(corev1alpha1.ObjectSetTemplatePhase{
		Name: "test-phase",
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm",
							"namespace": "default",
						},
					},
				},
			},
		},
	})

	// Create an object without conditions
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	actualObjects := []machinery.Object{obj}
	err := mapConditions(actualObjects, objectSetPhase)
	require.NoError(t, err)

	conds := *objectSetPhase.GetStatusConditions()
	assert.Empty(t, conds)
}

func TestMapConditions_MultipleObjects(t *testing.T) {
	t.Parallel()

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	objectSetPhase := adapters.NewObjectSetPhaseAccessor(scheme)

	// Setup phase with multiple objects and condition mappings
	objectSetPhase.SetPhase(corev1alpha1.ObjectSetTemplatePhase{
		Name: "test-phase",
		Objects: []corev1alpha1.ObjectSetObject{
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm-1",
							"namespace": "default",
						},
					},
				},
				ConditionMappings: []corev1alpha1.ConditionMapping{
					{
						SourceType:      "Ready",
						DestinationType: "example.com/CM1Ready",
					},
				},
			},
			{
				Object: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm-2",
							"namespace": "default",
						},
					},
				},
				ConditionMappings: []corev1alpha1.ConditionMapping{
					{
						SourceType:      "Available",
						DestinationType: "example.com/CM2Available",
					},
				},
			},
		},
	})

	// Create multiple objects with conditions
	obj1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "test-cm-1",
				"namespace":  "default",
				"generation": int64(1),
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":               "Ready",
						"status":             "True",
						"reason":             "AllGood",
						"message":            "CM1 is ready",
						"observedGeneration": int64(1),
					},
				},
			},
		},
	}

	obj2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":       "test-cm-2",
				"namespace":  "default",
				"generation": int64(1),
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":               "Available",
						"status":             "False",
						"reason":             "NotReady",
						"message":            "CM2 is not available",
						"observedGeneration": int64(1),
					},
				},
			},
		},
	}

	actualObjects := []machinery.Object{obj1, obj2}
	err := mapConditions(actualObjects, objectSetPhase)
	require.NoError(t, err)

	conds := *objectSetPhase.GetStatusConditions()
	assert.Len(t, conds, 2)

	// Find conditions by type
	var cm1Cond, cm2Cond *metav1.Condition
	for i := range conds {
		if conds[i].Type == "example.com/CM1Ready" {
			cm1Cond = &conds[i]
		}
		if conds[i].Type == "example.com/CM2Available" {
			cm2Cond = &conds[i]
		}
	}

	require.NotNil(t, cm1Cond)
	assert.Equal(t, metav1.ConditionTrue, cm1Cond.Status)
	assert.Equal(t, "AllGood", cm1Cond.Reason)

	require.NotNil(t, cm2Cond)
	assert.Equal(t, metav1.ConditionFalse, cm2Cond.Status)
	assert.Equal(t, "NotReady", cm2Cond.Reason)
}
