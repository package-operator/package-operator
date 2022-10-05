package objectsets

import (
	"context"
	"testing"

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

//func TestGenericObjectSetController_Reconcile(t *testing.T) {
//	controller, c, dc, pr, remotePr := getControllerAndMocks()
//
//	res, err := controller.Reconcile(context.Background(), ctrl.Request{})
//
//}

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
			controller, c, _, _, _ := getControllerAndMocks()
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
	controller, c, _, _, _ := getControllerAndMocks()
	c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.NewNotFound(schema.GroupResource{}, ""))
	os := &GenericObjectSet{}
	arePaused, unknown, err := controller.areRemotePhasesPaused(context.Background(), os)
	assert.False(t, arePaused)
	assert.True(t, unknown)
	assert.NoError(t, err)
}

func getControllerAndMocks() (*GenericObjectSetController, *testutil.CtrlClient, *dynamicCacheMock, *phaseReconcilerMock, *remotePhaseReconcilerMock) {
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
