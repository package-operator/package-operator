package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Recorder stores all Addon related metrics
type Recorder struct {
	addonsTotalAvailable *prometheus.GaugeVec
	addonsTotalPaused    *prometheus.GaugeVec
	addonsTotal          *prometheus.GaugeVec
	addonOperatorPaused  *prometheus.GaugeVec // 0 - Not paused , 1 - Paused
	// .. TODO: More metrics!
}

func NewRecorder() *Recorder {
	addonsAvailable := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "addon_operator_addons_available",
			Help: "Total number of Addons available",
		}, []string{})

	addonsPaused := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "addon_operator_addons_paused",
			Help: "Total number of Addons paused",
		}, []string{})

	addonsTotal := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "addon_operator_addons_total",
			Help: "Total number of Addon installations",
		}, []string{})

	addonOperatorPaused := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "addon_operator_paused",
			Help: "A boolean that tells if the AddonOperator is paused",
		}, []string{})

	// Register metrics
	ctrlmetrics.Registry.MustRegister(
		addonsTotal,
		addonsAvailable,
		addonsPaused,
		addonOperatorPaused,
	)

	return &Recorder{
		addonsTotal:          addonsTotal,
		addonsTotalAvailable: addonsAvailable,
		addonsTotalPaused:    addonsPaused,
		addonOperatorPaused:  addonOperatorPaused,
	}
}

func (r *Recorder) increaseAvailableAddonsCount() {
	r.addonsTotalAvailable.WithLabelValues().Inc()
}

func (r *Recorder) decreaseAvailableAddonsCount() {
	r.addonsTotalAvailable.WithLabelValues().Dec()
}

func (r *Recorder) increasePausedAddonsCount() {
	r.addonsTotalPaused.WithLabelValues().Inc()
}

func (r *Recorder) decreasePausedAddonsCount() {
	r.addonsTotalPaused.WithLabelValues().Dec()
}

func (r *Recorder) increaseTotalAddonsCount() {
	r.addonsTotal.WithLabelValues().Inc()
}

func (r *Recorder) decreaseTotalAddonsCount() {
	r.addonsTotal.WithLabelValues().Dec()
}

// SetAddonOperatorPaused sets the `addon_operator_paused` metric
// 0 - Not paused , 1 - Paused
func (r *Recorder) SetAddonOperatorPaused(paused bool) {
	if paused {
		r.addonOperatorPaused.WithLabelValues().Set(1)
	} else {
		r.addonOperatorPaused.WithLabelValues().Set(0)

	}
}

// HandleNewAddonInstallation increases the `addon_operator_addons_total` counter metric.
// It does this by initializing a State corresponding to the addonUID in the `stateObj`.
// This method is called at the top of the Reconciliation loop after `Get`-ing an Addon.
// If the Addon state already exists in `stateObj` the metric update is skipped.
func (r *Recorder) HandleNewAddonInstallation(addonUID string) {
	stateObj.mux.Lock()
	defer stateObj.mux.Unlock()
	_, ok := stateObj.conditionMapping[addonUID]

	// Check if Addon is not available in the internal mapping
	if !ok {
		// Initialize empty state condition.

		// These will later be updated in UpdateConditionMetrics
		// when the Conditions on the Addon are actually updated
		// by the controller after a successful reconciliation loop
		stateObj.conditionMapping[addonUID] = addonCondition{}

		r.increaseTotalAddonsCount()
	}
}

func (r *Recorder) HandleAddonUninstallation(addonUID string) {
	stateObj.mux.Lock()
	defer stateObj.mux.Unlock()
	conditions, ok := stateObj.conditionMapping[addonUID]

	// Check if Addon was found in the in-memory mapping
	if ok {
		r.decreaseTotalAddonsCount()

		// Reconcile the Condition metrics

		if conditions.available {
			r.decreaseAvailableAddonsCount()
		}

		if conditions.paused {
			r.decreasePausedAddonsCount()
		}

		// Delete entry in the in-memory map
		delete(stateObj.conditionMapping, addonUID)
	}
}

func (r *Recorder) UpdateConditionMetrics(addon *addonsv1alpha1.Addon) error {
	stateObj.mux.Lock()
	defer stateObj.mux.Unlock()

	uid := string(addon.UID)
	currState := addonCondition{
		available: meta.IsStatusConditionPresentAndEqual(addon.Status.Conditions,
			addonsv1alpha1.Available, metav1.ConditionTrue),
		paused: meta.IsStatusConditionPresentAndEqual(addon.Status.Conditions,
			addonsv1alpha1.Paused, metav1.ConditionTrue),
	}

	oldState, ok := stateObj.conditionMapping[uid]
	if !ok {
		// Addon should have already been initialized in the `stateObj`.
		// Return an error so that Reconciliation is retried and metrics are
		// updated successfully.
		return fmt.Errorf("failed to update metrics: could not sync Addon condition " +
			"with local in-memory mapping")
	}

	if oldState != currState {

		// Reconcile metrics with the current Conditions of the Addon

		if oldState.available != currState.available {
			if currState.available {
				r.increaseAvailableAddonsCount()
			} else {
				r.decreaseAvailableAddonsCount()
			}
		}

		if oldState.paused != currState.paused {
			if currState.paused {
				r.increasePausedAddonsCount()
			} else {
				r.decreasePausedAddonsCount()
			}
		}

		// Update the current Addon conditions in the in-memory map
		stateObj.conditionMapping[uid] = currState
	}
	return nil
}
