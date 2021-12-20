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
	recorder := NewRecorder(false)

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
		// addon_operator_addons_count{count_by="total"} 0
		assert.Equal(t, float64(0), testutil.ToFloat64(
			recorder.addonsCount.WithLabelValues(string(available))))
	})

	t.Run("new addon(s) installed", func(t *testing.T) {
		recorder.HandleAddonConditionAndInstallCount(
			addons[0].addonUID, addons[0].addonConditions, false)

		// Expected:
		// addon_operator_addons_count{count_by="total"} 1
		assert.Equal(t, float64(1), testutil.ToFloat64(
			recorder.addonsCount.WithLabelValues(string(total))))

		recorder.HandleAddonConditionAndInstallCount(addons[1].addonUID,
			addons[1].addonConditions, false)

		// Expected:
		// addon_operator_addons_count{count_by="total"} 2
		assert.Equal(t, float64(2), testutil.ToFloat64(
			recorder.addonsCount.WithLabelValues(string(total))))
	})

	t.Run("addon(s) uninstalled", func(t *testing.T) {
		recorder.HandleAddonConditionAndInstallCount(addons[0].addonUID, addons[0].addonConditions, true)

		// Expected:
		// addon_operator_addons_count{count_by="total"} 1
		assert.Equal(t, float64(1), testutil.ToFloat64(
			recorder.addonsCount.WithLabelValues(string(total))))

		recorder.HandleAddonConditionAndInstallCount(
			addons[1].addonUID, addons[1].addonConditions, true)

		// Expected:
		// addon_operator_addons_count{count_by="total"} 0
		assert.Equal(t, float64(0), testutil.ToFloat64(
			recorder.addonsCount.WithLabelValues(string(total))))
	})
}

func TestAddonMetrics_AddonConditions(t *testing.T) {
	recorder := NewRecorder(false)
	addonUID := "o672wxBaW9iR"

	t.Run("uninitialized conditions", func(t *testing.T) {
		conditions := []metav1.Condition{}
		recorder.HandleAddonConditionAndInstallCount(addonUID, conditions, false)

		// Expected:
		// addon_operator_addons_count{count_by="paused"} 0
		// addon_operator_addons_count{count_by="available"} 0
		assert.Equal(t, float64(0), testutil.ToFloat64(
			recorder.addonsCount.WithLabelValues(string(paused))))
		assert.Equal(t, float64(0), testutil.ToFloat64(
			recorder.addonsCount.WithLabelValues(string(available))))
	})

	// create a matrix of different combinations for available and paused
	for _, isAvailable := range []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse} {
		for _, isPaused := range []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse} {
			t.Run(fmt.Sprintf("addon available: %v, addon paused: %v", available, paused), func(t *testing.T) {
				expectedAvailable := 0
				if isAvailable == metav1.ConditionTrue {
					expectedAvailable = 1
				}
				expectedPaused := 0
				if isPaused == metav1.ConditionTrue {
					expectedPaused = 1
				}

				conditions := []metav1.Condition{
					{
						Type:   addonsv1alpha1.Available,
						Status: isAvailable,
					},
					{
						Type:   addonsv1alpha1.Paused,
						Status: isPaused,
					},
				}
				recorder.HandleAddonConditionAndInstallCount(addonUID, conditions, false)

				assert.Equal(t, float64(expectedPaused),
					testutil.ToFloat64(recorder.addonsCount.WithLabelValues(string(paused))))
				assert.Equal(t, float64(expectedAvailable),
					testutil.ToFloat64(recorder.addonsCount.WithLabelValues(string(available))))

			})
		}
	}

	t.Run("addon operator paused", func(t *testing.T) {
		recorder.SetAddonOperatorPaused(true)

		// Expected:
		// addon_operator_paused{} 1
		assert.Equal(t, float64(1), testutil.ToFloat64(recorder.addonOperatorPaused))
	})

	t.Run("addon operator unpaused", func(t *testing.T) {
		recorder.SetAddonOperatorPaused(false)

		// Expected:
		// addon_operator_paused{} 0
		assert.Equal(t, float64(0), testutil.ToFloat64(recorder.addonOperatorPaused))
	})
}
