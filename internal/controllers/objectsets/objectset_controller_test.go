package objectsets

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/testutil"
)

func TestGenericObjectSetController_Reconcile(t *testing.T) {
	//controller, c, dc, pr, remotePr := newControllerAndMocks()

	//res, err := controller.Reconcile(context.Background(), ctrl.Request{})

	tests := []struct {
		name                   string
		getObjectSetPhaseError error
		class                  string
		deletionTimestamp      *metav1.Time
		condition              metav1.Condition
	}{
		{
			name:                   "object doesn't exist",
			getObjectSetPhaseError: errors.NewNotFound(schema.GroupResource{}, ""),
			class:                  "",
			deletionTimestamp:      nil,
		},
		{
			name:                   "archived Condition",
			getObjectSetPhaseError: nil,
			class:                  "",
			deletionTimestamp:      nil,
			condition: metav1.Condition{
				Type:   corev1alpha1.ObjectSetArchived,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:                   "classes don't match",
			getObjectSetPhaseError: nil,
			class:                  "notDefault",
			deletionTimestamp:      nil,
		},
		{
			name:                   "already deleted",
			getObjectSetPhaseError: nil,
			class:                  "default",
			deletionTimestamp:      &metav1.Time{Time: time.Now()},
		},
		{
			name:                   "happy path",
			getObjectSetPhaseError: nil,
			class:                  "default",
			deletionTimestamp:      nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller, c, dc, pr, remotePr := newControllerAndMocks()

			//c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			//	Return(nil).Maybe()
			//c.StatusMock.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			//	Return(nil).Maybe()
			//
			//pr.On("Teardown", mock.Anything, mock.Anything).
			//	Return(true, nil).Once().Maybe()
			//pr.On("Reconcile", mock.Anything, mock.Anything).
			//	Return(ctrl.Result{}, nil).Maybe()
			//
			//dc.On("Free", mock.Anything, mock.Anything).Return(nil).Maybe()

			objectSetPhase := GenericObjectSet{}
			objectSetPhase.Spec.Class = test.class
			objectSetPhase.ClientObject().SetDeletionTimestamp(test.deletionTimestamp)
			objectSetPhase.Status.Conditions = []metav1.Condition{test.condition}
			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.ObjectSetPhase)
					objectSetPhase.DeepCopyInto(arg)
				}).
				Return(test.getObjectSetPhaseError)

			res, err := controller.Reconcile(context.Background(), ctrl.Request{})
			assert.Empty(t, res)
			assert.NoError(t, err)

			if test.getObjectSetPhaseError != nil || test.class != "default" {
				pr.AssertNotCalled(t, "Teardown", mock.Anything, mock.Anything)
				pr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				c.StatusMock.AssertNotCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}

			if test.deletionTimestamp != nil {
				pr.AssertCalled(t, "Teardown", mock.Anything, mock.Anything)
				pr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				dc.AssertCalled(t, "Free", mock.Anything, mock.Anything)
				c.StatusMock.AssertCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}

			// Happy path
			pr.AssertNotCalled(t, "Teardown", mock.Anything, mock.Anything)
			pr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
			c.StatusMock.AssertCalled(t, "Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestGenericObjectSetController_areRemotePhasesPaused_AllPhasesFound(t *testing.T) {
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
	pausedPhase2.Name = "pausedPhase1"
	pausedPhase2.Status.Conditions = []metav1.Condition{pausedCond}

	//[]corev1alpha1.RemotePhaseReference {
	//	return a.Status.RemotePhases
	tests := []struct {
		name     string
		phase1   corev1alpha1.ObjectSetPhase
		phase2   corev1alpha1.ObjectSetPhase
		expected bool
	}{
		{
			name:     "two unpaused phases",
			phase1:   unpausedPhase1,
			phase2:   unpausedPhase2,
			expected: true,
		},
		{
			name:     "one unpaused phase one paused phase",
			phase1:   pausedPhase1,
			phase2:   unpausedPhase2,
			expected: false,
		},
		{
			name:     "two paused phase",
			phase1:   pausedPhase1,
			phase2:   pausedPhase2,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

			os := &GenericObjectSet{}
			arePaused, unknown, err := controller.areRemotePhasesPaused(context.Background(), os)
			assert.Equal(t, test.expected, arePaused)
			assert.False(t, unknown)
			assert.NoError(t, err)
		})
	}
}

func TestGenericObjectSetController_areRemotePhasesPaused_PhaseNotFound(t *testing.T) {
	controller, c, _, _, _ := newControllerAndMocks()
	c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.NewNotFound(schema.GroupResource{}, ""))
	os := &GenericObjectSet{}
	arePaused, unknown, err := controller.areRemotePhasesPaused(context.Background(), os)
	assert.False(t, arePaused)
	assert.True(t, unknown)
	assert.NoError(t, err)
}

func newControllerAndMocks() (*GenericObjectSetController, *testutil.CtrlClient, *dynamicCacheMock, *phaseReconcilerMock, *remotePhaseReconcilerMock) {
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
	pr := &phaseReconcilerMock{}
	remotePr := &remotePhaseReconcilerMock{}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{}, nil
	}

	phasesReconciler := newObjectSetPhasesReconciler(pr, remotePr, lookup)

	controller.teardownHandler = phasesReconciler

	controller.reconciler = []reconciler{
		&revisionReconciler{
			scheme:       scheme,
			client:       c,
			newObjectSet: newGenericObjectSet,
		},
		phasesReconciler,
	}
	return controller, c, dc, pr, remotePr
}
