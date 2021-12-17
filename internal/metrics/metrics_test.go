package metrics

import (
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func TestAddonMetrics_InstallCount(t *testing.T) {
	recorder := NewRecorder()

	addons := []struct {
		addonUID        string
		addonConditions []metav1.Condition
	}{
		{
			addonUID:        "o672wxBaW9iR",
			addonConditions: []metav1.Condition{},
		},
		{
			addonUID:        "kpzLavSo27F8",
			addonConditions: []metav1.Condition{},
		},
	}

	t.Run("no addons installed", func(t *testing.T) {
		// Expected:
		// addon_operator_addons_total{} 0
		assert.Equal(t, float64(0), testutil.ToFloat64(recorder.addonsTotal.WithLabelValues()))
	})

	t.Run("new addon(s) installed", func(t *testing.T) {
		recorder.HandleAddonConditionAndInstallCount(addons[0].addonUID, addons[0].addonConditions, false)

		// Expected:
		// addon_operator_addons_total{} 1
		assert.Equal(t, float64(1), testutil.ToFloat64(recorder.addonsTotal.WithLabelValues()))

		recorder.HandleAddonConditionAndInstallCount(addons[1].addonUID, addons[1].addonConditions, false)

		// Expected:
		// addon_operator_addons_total{} 2
		assert.Equal(t, float64(2), testutil.ToFloat64(recorder.addonsTotal.WithLabelValues()))
	})

	t.Run("addon(s) uninstalled", func(t *testing.T) {
		recorder.HandleAddonConditionAndInstallCount(addons[0].addonUID, addons[0].addonConditions, true)

		// Expected:
		// addon_operator_addons_total{} 1
		assert.Equal(t, float64(1), testutil.ToFloat64(recorder.addonsTotal.WithLabelValues()))

		recorder.HandleAddonConditionAndInstallCount(addons[1].addonUID, addons[1].addonConditions, true)

		// Expected:
		// addon_operator_addons_total{} 2
		assert.Equal(t, float64(0), testutil.ToFloat64(recorder.addonsTotal.WithLabelValues()))
	})
}

func TestAddonMetrics_AddonConditions(t *testing.T) {
	recorder := NewRecorder()
	addonUID := "o672wxBaW9iR"

	t.Run("uninitialized conditions", func(t *testing.T) {
		conditions := []metav1.Condition{}
		recorder.HandleAddonConditionAndInstallCount(addonUID, conditions, false)

		// Expected:
		// addon_operator_addons_paused{} 0
		// addon_operator_addons_available{} 0
		assert.Equal(t, float64(0), testutil.ToFloat64(recorder.addonsTotalPaused.WithLabelValues()))
		assert.Equal(t, float64(0), testutil.ToFloat64(recorder.addonsTotalAvailable.WithLabelValues()))
	})

	// create a matrix of different combinations for available and paused
	for _, available := range []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse} {
		for _, paused := range []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse} {
			t.Run(fmt.Sprintf("addon available: %v, addon paused: %v", available, paused), func(t *testing.T) {
				expectedAvailable := 0
				if available == metav1.ConditionTrue {
					expectedAvailable = 1
				}
				expectedPaused := 0
				if paused == metav1.ConditionTrue {
					expectedPaused = 1
				}

				conditions := []metav1.Condition{
					{
						Type:   addonsv1alpha1.Available,
						Status: available,
					},
					{
						Type:   addonsv1alpha1.Paused,
						Status: paused,
					},
				}
				recorder.HandleAddonConditionAndInstallCount(addonUID, conditions, false)

				assert.Equal(t, float64(expectedPaused), testutil.ToFloat64(recorder.addonsTotalPaused.WithLabelValues()))
				assert.Equal(t, float64(expectedAvailable), testutil.ToFloat64(recorder.addonsTotalAvailable.WithLabelValues()))

			})
		}
	}

	t.Run("addon operator paused", func(t *testing.T) {
		recorder.SetAddonOperatorPaused(true)

		// Expected:
		// addon_operator_paused{} 1
		assert.Equal(t, float64(1), testutil.ToFloat64(recorder.addonOperatorPaused.WithLabelValues()))
	})

	t.Run("addon operator unpaused", func(t *testing.T) {
		recorder.SetAddonOperatorPaused(false)

		// Expected:
		// addon_operator_paused{} 0
		assert.Equal(t, float64(0), testutil.ToFloat64(recorder.addonOperatorPaused.WithLabelValues()))
	})
}
