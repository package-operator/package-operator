package metrics

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

// Recorder stores all the metrics related to Addons.
type Recorder struct {
	// metrics
	dynamicCacheSizeGvk     prometheus.Gauge
	dynamicCacheSizeObjects prometheus.Gauge
	rolloutTime             *prometheus.GaugeVec
	ocmAPIRequestDuration   prometheus.Summary // TODO: Keep this?
}

type GenericObjectSet interface {
	GetName() string
	GetConditions() *[]metav1.Condition
	GetCreationTimestamp() metav1.Time
}

func NewRecorder(register bool) *Recorder {

	dynamicCacheSizeGvk := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_size_gvk",
			Help: "Size of the dynamic cache in gvk's",
		})
	dynamicCacheSizeObjects := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_size_objects",
			Help: "Size of the dynamic cache in objects",
		})

	ocmAPIReqDuration := prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name: "addon_operator_ocm_api_requests_durations",
			Help: "OCM API request latencies in microseconds",
			// p50, p90 and p99 latencies
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		})

	rolloutTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_rollout_time_seconds",
			Help: "Rollout time",
		}, []string{"name"},
	)

	// Register metrics if `register` is true
	// This allows us to skip registering metrics
	// and re-use the recorder when testing.
	if register {
		ctrlmetrics.Registry.MustRegister(
			dynamicCacheSizeGvk,
			dynamicCacheSizeObjects,
			ocmAPIReqDuration,
			rolloutTime,
		)
	}

	return &Recorder{
		dynamicCacheSizeGvk:     dynamicCacheSizeGvk,
		dynamicCacheSizeObjects: dynamicCacheSizeObjects,
		ocmAPIRequestDuration:   ocmAPIReqDuration,
		rolloutTime:             rolloutTime,
	}
}

func (r *Recorder) RecordRolloutTime(os GenericObjectSet) {
	start := os.GetCreationTimestamp()
	conds := os.GetConditions()
	for _, cond := range *conds {
		if cond.Type == "Success" {
			r.rolloutTime.WithLabelValues(os.GetName()).Set(cond.LastTransitionTime.Sub(start.Time).Seconds())
		}
	}
}

func (r *Recorder) RecordDynamicCacheSizeGvk(count int) {
	r.dynamicCacheSizeGvk.Set(float64(count))
}

func (r *Recorder) RecordDynamicCacheSizeObj(count int) {
	r.dynamicCacheSizeObjects.Set(float64(count))
}

// InjectOCMAPIRequestDuration allows us to override `r.ocmAPIRequestDuration` metric
// Useful while writing tests.
func (r *Recorder) InjectOCMAPIRequestDuration(s prometheus.Summary) {
	r.ocmAPIRequestDuration = s
}

func (r *Recorder) RecordOCMAPIRequests(us float64) {
	r.ocmAPIRequestDuration.Observe(us)
}
