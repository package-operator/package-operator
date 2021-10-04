package controllers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/addon-operator/internal/testutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func TestHandleAddonOperatorPause(t *testing.T) {
	for i, tc := range []bool{true, false} {
		tc := tc //pin
		t.Run(fmt.Sprintf("test case %d: addonoperator.spec.paused: %v", i, tc),
			func(t *testing.T) {
				testHandlePause(t, tc)
			})
	}
}

func setPauseConditionOnAddonOperator(addonOperator *addonsv1alpha1.AddonOperator) {
	meta.SetStatusCondition(&addonOperator.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Paused,
		Status:             metav1.ConditionTrue,
		Reason:             addonsv1alpha1.AddonOperatorReasonPaused,
		Message:            "Addon operator is paused",
		ObservedGeneration: addonOperator.Generation,
	})
}

func testHandlePause(t *testing.T, paused bool) {
	c := testutil.NewClient()
	assertFunc := assert.True
	checkStatusCondition := meta.IsStatusConditionTrue

	addonOperator := newAddonOperatorWithPause(paused)
	if !paused {
		// the reconciler tries to unpause only
		// when spec.paused is set to `false` and
		// Paused condition is being reported
		setPauseConditionOnAddonOperator(addonOperator)
		assertFunc = assert.False
		checkStatusCondition = meta.IsStatusConditionFalse
	}

	c.On("List", testutil.IsContext,
		testutil.IsAddonsv1alpha1AddonListPtr,
		mock.Anything).
		Return(nil)
	c.StatusMock.On("Update", testutil.IsContext,
		testutil.IsAddonsv1alpha1AddonPtr,
		mock.Anything).
		Return(nil)
	c.StatusMock.On("Update", testutil.IsContext,
		testutil.IsAddonsv1alpha1AddonOperatorPtr,
		mock.Anything).
		Return(nil)

	r, pauseManager := newAddonOperatorReconciler(c, testutil.NewLogger(t))

	pauseManager.globalPauseMux.RLock()
	defer pauseManager.globalPauseMux.RUnlock()
	isPaused := pauseManager.globalPause

	ctx := context.Background()
	requeue, err := r.handleGlobalPause(ctx, addonOperator)
	addonOperatorPaused := checkStatusCondition(addonOperator.Status.Conditions,
		addonsv1alpha1.Paused)

	require.NoError(t, err)
	assertFunc(t, requeue)
	assertFunc(t, isPaused)
	assertFunc(t, addonOperatorPaused)
	c.AssertExpectations(t)
}
