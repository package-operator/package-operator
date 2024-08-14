package objectsetphases

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/ownerhandling"
	"package-operator.run/internal/testutil"
)

type dynamicCacheMock struct {
	testutil.CtrlClient
}

func (c *dynamicCacheMock) Watch(
	ctx context.Context, owner client.Object, obj runtime.Object,
) error {
	args := c.Called(ctx, owner, obj)
	return args.Error(0)
}

func (c *dynamicCacheMock) Source(handler handler.EventHandler, predicates ...predicate.Predicate) source.Source {
	args := c.Called(handler, predicates)
	return args.Get(0).(source.Source)
}

func (c *dynamicCacheMock) Free(ctx context.Context, obj client.Object) error {
	args := c.Called(ctx, obj)
	return args.Error(0)
}

type objectSetPhaseReconcilerMock struct {
	mock.Mock
}

func (c *objectSetPhaseReconcilerMock) Reconcile(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) (res ctrl.Result, err error) {
	args := c.Called(ctx, objectSetPhase)
	return args.Get(0).(ctrl.Result), args.Error(1)
}

func (c *objectSetPhaseReconcilerMock) Teardown(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) (cleanupDone bool, err error) {
	args := c.Called(ctx, objectSetPhase)
	return args.Bool(0), args.Error(1)
}

func newControllerAndMocks() (
	*GenericObjectSetPhaseController, *testutil.CtrlClient, *dynamicCacheMock, *objectSetPhaseReconcilerMock,
) {
	dc := &dynamicCacheMock{}

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	c := testutil.NewClient()
	// NewSameClusterObjectSetPhaseController
	controller := &GenericObjectSetPhaseController{
		newObjectSetPhase: newGenericObjectSetPhase,

		class:  "default",
		log:    ctrl.Log.WithName("controllers"),
		scheme: scheme,

		client:        c,
		dynamicCache:  dc,
		ownerStrategy: ownerhandling.NewNative(scheme),
	}

	pr := &objectSetPhaseReconcilerMock{}
	controller.teardownHandler = pr
	controller.reconciler = []reconciler{
		pr,
	}

	return controller, c, dc, pr
}

func TestGenericObjectSetPhaseController_Reconcile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		getObjectSetPhaseError error
		class                  string
		deletionTimestamp      *metav1.Time
	}{
		{
			name:                   "object doesn't exist",
			getObjectSetPhaseError: errors.NewNotFound(schema.GroupResource{}, ""),
			class:                  "",
			deletionTimestamp:      nil,
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
			name:                   "runs all the way through",
			getObjectSetPhaseError: nil,
			class:                  "default",
			deletionTimestamp:      nil,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			controller, c, dc, pr := newControllerAndMocks()

			c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			c.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()

			pr.On("Teardown", mock.Anything, mock.Anything).
				Return(true, nil).Once().Maybe()
			pr.On("Reconcile", mock.Anything, mock.Anything).
				Return(ctrl.Result{}, nil).Maybe()

			dc.On("Free", mock.Anything, mock.Anything).Return(nil).Maybe()

			objectSetPhase := GenericObjectSetPhase{}
			objectSetPhase.Finalizers = []string{
				constants.CachedFinalizer,
			}
			objectSetPhase.Labels = map[string]string{
				corev1alpha1.ObjectSetPhaseClassLabel: test.class,
			}
			objectSetPhase.ClientObject().SetDeletionTimestamp(test.deletionTimestamp)
			c.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					arg := args.Get(2).(*corev1alpha1.ObjectSetPhase)
					objectSetPhase.DeepCopyInto(arg)
				}).
				Return(test.getObjectSetPhaseError)

			res, err := controller.Reconcile(context.Background(), ctrl.Request{})
			assert.Empty(t, res)
			require.NoError(t, err)

			if test.getObjectSetPhaseError != nil || test.class != "default" {
				pr.AssertNotCalled(t, "Teardown", mock.Anything, mock.Anything)
				pr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				c.StatusMock.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}

			if test.deletionTimestamp != nil {
				pr.AssertCalled(t, "Teardown", mock.Anything, mock.Anything)
				pr.AssertNotCalled(t, "Reconcile", mock.Anything, mock.Anything)
				dc.AssertCalled(t, "Free", mock.Anything, mock.Anything)
				c.StatusMock.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				return
			}

			pr.AssertNotCalled(t, "Teardown", mock.Anything, mock.Anything)
			pr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything)
			c.StatusMock.AssertCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestGenericObjectSetPhaseController_handleDeletionAndArchival(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		teardownDone bool
	}{
		{
			name:         "teardown not done",
			teardownDone: false,
		},
		{
			name:         "teardown done",
			teardownDone: true,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			controller, client, dc, pr := newControllerAndMocks()

			pr.On("Teardown", mock.Anything, mock.Anything).
				Return(test.teardownDone, nil).Maybe()
			client.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			dc.On("Free", mock.Anything, mock.Anything).
				Return(nil).Maybe()

			err := controller.handleDeletionAndArchival(context.Background(), &GenericObjectSetPhase{
				ObjectSetPhase: corev1alpha1.ObjectSetPhase{
					ObjectMeta: metav1.ObjectMeta{Finalizers: []string{
						constants.CachedFinalizer,
					}},
				},
			})
			require.NoError(t, err)
			if test.teardownDone {
				dc.AssertCalled(t, "Free", mock.Anything, mock.Anything)
			} else {
				dc.AssertNotCalled(t, "Free", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestGenericObjectSetPhaseController_reportPausedCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		phasePaused        bool
		startingConditions []metav1.Condition
	}{
		{
			name:        "phase pause",
			phasePaused: true,
		},
		{
			name:        "phase not paused but has paused condition",
			phasePaused: false,
			startingConditions: []metav1.Condition{
				{
					Type:   corev1alpha1.ObjectSetPaused,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controller, _, _, _ := newControllerAndMocks()

			p := &GenericObjectSetPhase{}
			p.Spec.Paused = test.phasePaused
			p.Status.Conditions = test.startingConditions

			controller.reportPausedCondition(context.Background(), p)
			conds := *p.GetConditions()
			if test.phasePaused {
				assert.Len(t, conds, 1)
				assert.Equal(t, corev1alpha1.ObjectSetPhasePaused, conds[0].Type)
				assert.Equal(t, metav1.ConditionTrue, conds[0].Status)
			} else {
				assert.Empty(t, conds)
			}
		})
	}
}

func TestInitializers(t *testing.T) {
	t.Parallel()

	log := testr.New(t)
	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	dc := &dynamicCacheMock{}
	client := testutil.NewClient()
	class := "default"
	mapper := meta.NewDefaultRESTMapper(scheme.PreferredVersionAllGroups())

	t.Run("NewMultiClusterObjectSetPhaseController", func(t *testing.T) {
		t.Parallel()

		ctrl := NewMultiClusterObjectSetPhaseController(
			log, scheme,
			dc, client, class, client, client,
			mapper,
		)

		require.NotNil(t, ctrl)
	})

	t.Run("NewMultiClusterClusterObjectSetPhaseController", func(t *testing.T) {
		t.Parallel()

		ctrl := NewMultiClusterClusterObjectSetPhaseController(
			log, scheme,
			dc, client, class, client, client,
			mapper,
		)

		require.NotNil(t, ctrl)
	})

	t.Run("NewSameClusterObjectSetPhaseController", func(t *testing.T) {
		t.Parallel()

		ctrl := NewSameClusterObjectSetPhaseController(
			log, scheme,
			dc, client, class, client,
			mapper,
		)

		require.NotNil(t, ctrl)
	})

	t.Run("NewSameClusterClusterObjectSetPhaseController", func(t *testing.T) {
		t.Parallel()

		ctrl := NewSameClusterClusterObjectSetPhaseController(
			log, scheme,
			dc, client, class, client,
			mapper,
		)

		require.NotNil(t, ctrl)
	})
}
