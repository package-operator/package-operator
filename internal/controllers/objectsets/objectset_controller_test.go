package objectsets

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/preflight"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/dynamiccachemocks"
)

type dynamicCacheMock = dynamiccachemocks.DynamicCacheMock

type objectSetPhasesReconcilerMock struct {
	mock.Mock
}

func (r *objectSetPhasesReconcilerMock) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (res ctrl.Result, err error) {
	args := r.Called(ctx, objectSet)
	return args.Get(0).(ctrl.Result), args.Error(1)
}

func (r *objectSetPhasesReconcilerMock) Teardown(
	ctx context.Context, objectSet genericObjectSet,
) (cleanupDone bool, err error) {
	args := r.Called(ctx, objectSet)
	return args.Bool(0), args.Error(1)
}

type revisionReconcilerMock struct {
	mock.Mock
}

func (r *revisionReconcilerMock) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (res ctrl.Result, err error) {
	args := r.Called(ctx, objectSet)
	return args.Get(0).(ctrl.Result), args.Error(1)
}

func TestGenericObjectSetController_Reconcile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		getObjectSetPhaseError error
		deletionTimestamp      *metav1.Time
		condition              metav1.Condition
		lifecycleState         corev1alpha1.ObjectSetLifecycleState
	}{
		{
			name:                   "objectset does not exist",
			getObjectSetPhaseError: apimachineryerrors.NewNotFound(schema.GroupResource{}, ""),
		},
		{
			name: "archived condition",
			condition: metav1.Condition{
				Type:   corev1alpha1.ObjectSetArchived,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:           "archived lifecyclestate",
			lifecycleState: corev1alpha1.ObjectSetLifecycleStateArchived,
		},
		{
			name:              "already deleted",
			deletionTimestamp: &metav1.Time{Time: time.Now()},
		},
		{
			name: "run all the way through",
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controller, c, dc, pr, rr := newControllerAndMocks()

			c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			c.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()

			pr.On("Reconcile", mock.Anything, mock.Anything).
				Return(ctrl.Result{}, nil).Maybe()
			pr.On("Teardown", mock.Anything, mock.Anything).
				Return(true, nil).Once().Maybe()

			rr.On("Reconcile", mock.Anything, mock.Anything).
				Return(ctrl.Result{}, nil).Maybe()
			rr.On("Teardown", mock.Anything).
				Return(true, nil).Once().Maybe()

			dc.On("Free", mock.Anything, mock.Anything).Return(nil).Maybe()

			objectSet := GenericObjectSet{
				ObjectSet: corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{
							controllers.CachedFinalizer,
						},
					},
				},
			}
			objectSet.ClientObject().SetDeletionTimestamp(test.deletionTimestamp)
			objectSet.Status.Conditions = []metav1.Condition{test.condition}
			objectSet.Spec.LifecycleState = test.lifecycleState

			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.ObjectSet)
					objectSet.DeepCopyInto(arg)
				}).
				Return(test.getObjectSetPhaseError)

			res, err := controller.Reconcile(context.Background(), ctrl.Request{})
			assert.Empty(t, res)
			assert.NoError(t, err)

			if test.getObjectSetPhaseError != nil || test.condition.Type == corev1alpha1.ObjectSetArchived {
				pr.AssertNotCalled(t, "Teardown", mock.Anything, mock.Anything)
				pr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				c.StatusMock.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}

			if test.deletionTimestamp != nil || test.lifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived {
				pr.AssertCalled(t, "Teardown", mock.Anything, mock.Anything)
				pr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				dc.AssertCalled(t, "Free", mock.Anything, mock.Anything)
				if test.deletionTimestamp == nil {
					c.StatusMock.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				}
				return
			}

			//  "run all the way through"
			pr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
			pr.AssertNotCalled(t, "Teardown", mock.Anything, mock.Anything)
			rr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
			rr.AssertNotCalled(t, "Teardown", mock.Anything, mock.Anything)
			dc.AssertNotCalled(t, "Free", mock.Anything, mock.Anything)
			c.StatusMock.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestGenericObjectSetController_areRemotePhasesPaused_AllPhasesFound(t *testing.T) {
	t.Parallel()

	pausedCond := metav1.Condition{
		Type:   corev1alpha1.ObjectSetPaused,
		Status: metav1.ConditionTrue,
	}
	unpausedPhase1 := corev1alpha1.ObjectSetPhase{}
	unpausedPhase1.Name = "unpausedPhase1"
	unpausedPhase2 := corev1alpha1.ObjectSetPhase{}
	unpausedPhase2.Name = "unpausedPhase2"
	pausedPhase1 := corev1alpha1.ObjectSetPhase{}
	pausedPhase1.Name = "pausedPhase1"
	pausedPhase1.Status.Conditions = []metav1.Condition{pausedCond}
	pausedPhase2 := corev1alpha1.ObjectSetPhase{}
	pausedPhase2.Name = "pausedPhase2"
	pausedPhase2.Status.Conditions = []metav1.Condition{pausedCond}

	tests := []struct {
		name              string
		phase1            corev1alpha1.ObjectSetPhase
		phase2            corev1alpha1.ObjectSetPhase
		arePausedExpected bool
	}{
		{
			name:              "two unpaused phases",
			phase1:            unpausedPhase1,
			phase2:            unpausedPhase2,
			arePausedExpected: false,
		},
		{
			name:              "one unpaused phase one paused phase",
			phase1:            pausedPhase1,
			phase2:            unpausedPhase2,
			arePausedExpected: false,
		},
		{
			name:              "two paused phase",
			phase1:            pausedPhase1,
			phase2:            pausedPhase2,
			arePausedExpected: true,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controller, c, _, _, _ := newControllerAndMocks()
			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.ObjectSetPhase)
					test.phase1.DeepCopyInto(arg)
				}).
				Return(nil).Once()

			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.ObjectSetPhase)
					test.phase2.DeepCopyInto(arg)
				}).
				Return(nil).Once()

			objectSet := &GenericObjectSet{}
			objectSet.Status.RemotePhases = make([]corev1alpha1.RemotePhaseReference, 2)
			arePaused, unknown, err := controller.areRemotePhasesPaused(context.Background(), objectSet)
			assert.Equal(t, test.arePausedExpected, arePaused)
			assert.False(t, unknown)
			assert.NoError(t, err)
		})
	}
}

func TestGenericObjectSetController_areRemotePhasesPaused_PhaseNotFound(t *testing.T) {
	t.Parallel()
	controller, c, _, _, _ := newControllerAndMocks()
	c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))
	objectSet := &GenericObjectSet{}
	objectSet.Status.RemotePhases = make([]corev1alpha1.RemotePhaseReference, 2)
	arePaused, unknown, err := controller.areRemotePhasesPaused(context.Background(), objectSet)
	assert.False(t, arePaused)
	assert.True(t, unknown)
	assert.NoError(t, err)
}

func TestGenericObjectSetController_areRemotePhasesPaused_reportPausedCondition(t *testing.T) {
	t.Parallel()

	pausedCond := metav1.Condition{
		Type:   corev1alpha1.ObjectSetPaused,
		Status: metav1.ConditionTrue,
	}
	unpausedPhase := corev1alpha1.ObjectSetPhase{}
	pausedPhase := corev1alpha1.ObjectSetPhase{}
	pausedPhase.Status.Conditions = []metav1.Condition{pausedCond}

	tests := []struct {
		name                  string
		phase                 corev1alpha1.ObjectSetPhase
		getPhaseError         error
		objectSetPaused       bool
		pausedConditionStatus metav1.ConditionStatus
		startingConditions    []metav1.Condition
	}{
		{
			name:                  "areRemotePhasesPaused unknown",
			getPhaseError:         apimachineryerrors.NewNotFound(schema.GroupResource{}, ""),
			pausedConditionStatus: metav1.ConditionUnknown,
		},
		{
			name:                  "areRemotePhasesPaused true, ObjectSet isPaused true",
			objectSetPaused:       true,
			phase:                 pausedPhase,
			pausedConditionStatus: metav1.ConditionTrue,
		},
		{
			name:               "areRemotePhasesPaused false",
			objectSetPaused:    false,
			phase:              unpausedPhase,
			startingConditions: []metav1.Condition{pausedCond},
		},
		{
			name:                  "areRemotePhasesPaused true, ObjectSet isPaused true",
			objectSetPaused:       true,
			phase:                 unpausedPhase,
			pausedConditionStatus: metav1.ConditionUnknown,
		},
		{
			name:                  "areRemotePhasesPaused true, ObjectSet isPaused false",
			objectSetPaused:       false,
			phase:                 pausedPhase,
			pausedConditionStatus: metav1.ConditionUnknown,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controller, c, _, _, _ := newControllerAndMocks()
			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.ObjectSetPhase)
					test.phase.DeepCopyInto(arg)
				}).
				Return(test.getPhaseError).Once()

			objectSet := &GenericObjectSet{}
			objectSet.Status.RemotePhases = make([]corev1alpha1.RemotePhaseReference, 1)
			if test.objectSetPaused {
				objectSet.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
			}
			objectSet.Status.Conditions = test.startingConditions
			err := controller.reportPausedCondition(context.Background(), objectSet)
			assert.NoError(t, err)
			conds := *objectSet.GetConditions()
			if test.pausedConditionStatus != "" {
				assert.Len(t, conds, 1)
				assert.Equal(t, corev1alpha1.ObjectSetPaused, conds[0].Type)
				assert.Equal(t, test.pausedConditionStatus, conds[0].Status)
			} else {
				assert.Len(t, conds, 0)
			}
		})
	}
}

func TestGenericObjectSetController_handleDeletionAndArchival(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		teardownDone            bool
		lifecycleState          corev1alpha1.ObjectSetLifecycleState
		archivedConditionStatus metav1.ConditionStatus
	}{
		{
			name:                    "teardown not done and archived lifecycle state",
			teardownDone:            false,
			lifecycleState:          corev1alpha1.ObjectSetLifecycleStateArchived,
			archivedConditionStatus: metav1.ConditionFalse,
		},
		{
			name:                    "teardown done and archived lifecycle state",
			teardownDone:            true,
			lifecycleState:          corev1alpha1.ObjectSetLifecycleStateArchived,
			archivedConditionStatus: metav1.ConditionTrue,
		},
		{
			name:         "teardown done and no lifecycle state",
			teardownDone: false,
		},
		{
			name:         "teardown done and no lifecycle state",
			teardownDone: true,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controller, client, dc, pr, _ := newControllerAndMocks()

			pr.On("Teardown", mock.Anything, mock.Anything).
				Return(test.teardownDone, nil).Maybe()
			dc.On("Free", mock.Anything, mock.Anything).Return(nil).Maybe()
			client.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			objectSet := &GenericObjectSet{
				ObjectSet: corev1alpha1.ObjectSet{
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: []string{
							controllers.CachedFinalizer,
						},
					},
				},
			}
			objectSet.Spec.LifecycleState = test.lifecycleState
			objectSet.Status.Conditions = []metav1.Condition{
				{
					Type:   corev1alpha1.ObjectSetAvailable,
					Status: metav1.ConditionTrue,
				},
			}

			err := controller.handleDeletionAndArchival(context.Background(), objectSet)
			assert.NoError(t, err)
			conds := *objectSet.GetConditions()

			if test.teardownDone {
				dc.AssertCalled(t, "Free", mock.Anything, mock.Anything)
			} else {
				dc.AssertNotCalled(t, "Free", mock.Anything, mock.Anything)
			}

			if test.lifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived {
				assert.Len(t, conds, 1)
				assert.Equal(t, conds[0].Type, corev1alpha1.ObjectSetArchived)
				assert.Equal(t, conds[0].Status, test.archivedConditionStatus)
			} else {
				assert.Len(t, conds, 0)
			}
		})
	}
}

var errTest = errors.New("explosion")

func TestGenericObjectSetController_updateStatusError(t *testing.T) {
	t.Parallel()

	t.Run("just returns error", func(t *testing.T) {
		t.Parallel()

		objectSet := &GenericObjectSet{
			ObjectSet: corev1alpha1.ObjectSet{},
		}

		c, _, _, _, _ := newControllerAndMocks()
		ctx := context.Background()
		_, err := controllers.UpdateObjectSetOrPhaseStatusFromError(ctx, objectSet, errTest,
			func(ctx context.Context) error {
				return c.updateStatus(ctx, objectSet)
			})
		assert.EqualError(t, err, "explosion")
	})

	t.Run("reports preflight error", func(t *testing.T) {
		t.Parallel()

		objectSet := &GenericObjectSet{
			ObjectSet: corev1alpha1.ObjectSet{},
		}

		c, client, _, _, _ := newControllerAndMocks()

		client.StatusMock.
			On("Update", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)

		ctx := context.Background()
		_, err := controllers.UpdateObjectSetOrPhaseStatusFromError(ctx, objectSet, &preflight.Error{},
			func(ctx context.Context) error {
				return c.updateStatus(ctx, objectSet)
			})
		require.NoError(t, err)

		client.StatusMock.AssertExpectations(t)
	})
}

func newControllerAndMocks() (
	*GenericObjectSetController,
	*testutil.CtrlClient,
	*dynamicCacheMock,
	*objectSetPhasesReconcilerMock,
	*revisionReconcilerMock,
) {
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	c := testutil.NewClient()
	dc := &dynamicCacheMock{}

	controller := &GenericObjectSetController{
		newObjectSet:      newGenericObjectSet,
		newObjectSetPhase: newGenericObjectSetPhase,
		client:            c,
		log:               ctrl.Log.WithName("controllers"),
		scheme:            scheme,
		dynamicCache:      dc,
	}
	pr := &objectSetPhasesReconcilerMock{}

	controller.teardownHandler = pr

	rr := &revisionReconcilerMock{}
	controller.reconciler = []reconciler{
		rr,
		pr,
	}
	return controller, c, dc, pr, rr
}
