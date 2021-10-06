package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestHandleAddonOperatorPause_(t *testing.T) {
	t.Run("enables global pause", func(t *testing.T) {
		c := testutil.NewClient()
		gpm := &globalPauseManagerMock{}
		r := &AddonOperatorReconciler{
			Client:             c,
			GlobalPauseManager: gpm,
		}
		ctx := context.Background()
		ao := &addonsv1alpha1.AddonOperator{
			Spec: addonsv1alpha1.AddonOperatorSpec{
				Paused: true,
			},
		}

		gpm.On("EnableGlobalPause", mock.Anything).Return(nil)
		c.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		err := r.handleGlobalPause(ctx, ao)
		require.NoError(t, err)

		gpm.AssertCalled(t, "EnableGlobalPause", mock.Anything)

		pausedCond := meta.FindStatusCondition(ao.Status.Conditions, addonsv1alpha1.Paused)
		if assert.NotNil(t, pausedCond, "Paused condition should be present on AddonOperator object") {
			assert.Equal(t, metav1.ConditionTrue, pausedCond.Status)
		}
	})

	t.Run("does not enable pause twice when status is already reported", func(t *testing.T) {
		c := testutil.NewClient()
		gpm := &globalPauseManagerMock{}
		r := &AddonOperatorReconciler{
			Client:             c,
			GlobalPauseManager: gpm,
		}
		ctx := context.Background()
		ao := &addonsv1alpha1.AddonOperator{
			Spec: addonsv1alpha1.AddonOperatorSpec{
				Paused: true,
			},
			Status: addonsv1alpha1.AddonOperatorStatus{
				Conditions: []metav1.Condition{
					{
						Type:   addonsv1alpha1.Paused,
						Status: metav1.ConditionTrue,
					},
				},
			},
		}

		gpm.On("EnableGlobalPause", mock.Anything).Return(nil)
		c.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		err := r.handleGlobalPause(ctx, ao)
		require.NoError(t, err)

		// When status is already reported, don't EnableGlobalPause again.
		gpm.AssertNotCalled(t, "EnableGlobalPause", mock.Anything)
	})

	t.Run("disables global pause", func(t *testing.T) {
		c := testutil.NewClient()
		gpm := &globalPauseManagerMock{}
		r := &AddonOperatorReconciler{
			Client:             c,
			GlobalPauseManager: gpm,
		}
		ctx := context.Background()
		ao := &addonsv1alpha1.AddonOperator{
			Spec: addonsv1alpha1.AddonOperatorSpec{
				Paused: false,
			},
			Status: addonsv1alpha1.AddonOperatorStatus{
				Conditions: []metav1.Condition{
					{
						Type:   addonsv1alpha1.Paused,
						Status: metav1.ConditionTrue,
					},
				},
			},
		}

		gpm.On("DisableGlobalPause", mock.Anything).Return(nil)
		c.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		err := r.handleGlobalPause(ctx, ao)
		require.NoError(t, err)

		gpm.AssertCalled(t, "DisableGlobalPause", mock.Anything)
		pausedCond := meta.FindStatusCondition(ao.Status.Conditions, addonsv1alpha1.Paused)
		assert.Nil(t, pausedCond, "Paused condition should be removed on AddonOperator object")
	})

	t.Run("does not disable twice when status is already reported", func(t *testing.T) {
		c := testutil.NewClient()
		gpm := &globalPauseManagerMock{}
		r := &AddonOperatorReconciler{
			Client:             c,
			GlobalPauseManager: gpm,
		}
		ctx := context.Background()
		ao := &addonsv1alpha1.AddonOperator{
			Spec: addonsv1alpha1.AddonOperatorSpec{
				Paused: false,
			},
		}

		gpm.On("DisableGlobalPause", mock.Anything).Return(nil)
		c.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		err := r.handleGlobalPause(ctx, ao)
		require.NoError(t, err)

		// When status is gone, don't DisableGlobalPause again.
		gpm.AssertNotCalled(t, "DisableGlobalPause", mock.Anything)
	})
}

type globalPauseManagerMock struct {
	mock.Mock
}

func (r *globalPauseManagerMock) EnableGlobalPause(ctx context.Context) error {
	args := r.Called(ctx)
	return args.Error(0)
}

func (r *globalPauseManagerMock) DisableGlobalPause(ctx context.Context) error {
	args := r.Called(ctx)
	return args.Error(0)
}
