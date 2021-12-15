package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

const prefix = "addon_operator_"

func prefixedMetricName(name string) string {
	return fmt.Sprintf("%s%s", prefix, name)
}

// Recorder stores all Addon related metrics
type Recorder struct {
	addonsAvailable *prometheus.GaugeVec
	addonsPaused    *prometheus.GaugeVec
	addonsTotal     *prometheus.GaugeVec
	// .. TODO: More metrics!
}

func NewRecorder() *Recorder {
	addonsAvailable := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: prefixedMetricName("addons_available"),
			Help: "Total number of Addons available",
		}, []string{})

	addonsPaused := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: prefixedMetricName("addons_paused"),
			Help: "Total number of Addons paused",
		}, []string{})

	addonsTotal := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: prefixedMetricName("addons_total"),
			Help: "Total number of Addon instances",
		}, []string{})

	// Register metrics
	ctrlmetrics.Registry.MustRegister(
		addonsTotal,
		addonsAvailable,
		addonsPaused,
	)

	return &Recorder{
		addonsTotal:     addonsTotal,
		addonsAvailable: addonsAvailable,
		addonsPaused:    addonsPaused,
	}
}

func (r *Recorder) increaseAvailableCount() {
	r.addonsAvailable.WithLabelValues().Inc()
}

func (r *Recorder) decreaseAvailableCount() {
	r.addonsAvailable.WithLabelValues().Dec()
}

func (r *Recorder) increasePausedCount() {
	r.addonsPaused.WithLabelValues().Inc()
}

func (r *Recorder) decreasePausedCount() {
	r.addonsPaused.WithLabelValues().Dec()
}

func (r *Recorder) UpdateConditionMetrics(addon *addonsv1alpha1.Addon) {
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
		// create a new entry
		stateObj.conditionMapping[uid] = currState
		if currState.available {
			// increase available metric
			r.increasePausedCount()
		}

		if currState.paused {
			// increase paused metric
			r.increasePausedCount()
		}
		return
	}

	// available condition changed
	if oldState.available != currState.available {
		// update metric according to current condition
		if currState.available {
			r.increaseAvailableCount()
		} else {
			r.decreaseAvailableCount()
		}
	}

	// paused condition changed
	if oldState.paused != currState.paused {
		// update metric according to current condition
		if currState.paused {
			r.increasePausedCount()
		} else {
			r.decreasePausedCount()
		}
	}

	if oldState != currState {
		stateObj.conditionMapping[uid] = currState
	}
}
