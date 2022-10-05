package metrics

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

// Recorder stores all the metrics related to Addons.
type Recorder struct {
	dynamicCacheSizeGvk       prometheus.Gauge
	dynamicCacheSizeObjects   prometheus.Gauge
	objectSetRolloutTime      *prometheus.GaugeVec
	objectSetPhaseRolloutTime *prometheus.GaugeVec
}

type GenericObjectSet interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
}

type GenericObjectSetPhase interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
}

func NewRecorder(register bool) *Recorder {

	dynamicCacheSizeGvk := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_size_gvks",
			Help: "Size of the dynamic cache in gvk's",
		})
	dynamicCacheSizeObjects := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_size_objects",
			Help: "Size of the dynamic cache in objects",
		})
	objectSetRolloutTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_object_set_rollout_time_seconds",
			Help: "Rollout time",
		}, []string{"name"},
	)
	objectSetPhaseRolloutTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_object_set_phase_rollout_time_seconds",
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
			objectSetRolloutTime,
			objectSetPhaseRolloutTime,
		)
	}

	return &Recorder{
		dynamicCacheSizeGvk:       dynamicCacheSizeGvk,
		dynamicCacheSizeObjects:   dynamicCacheSizeObjects,
		objectSetRolloutTime:      objectSetRolloutTime,
		objectSetPhaseRolloutTime: objectSetPhaseRolloutTime,
	}
}

func (r *Recorder) RecordRolloutTimeObjectSet(objectSet GenericObjectSet) {
	obj := objectSet.ClientObject()
	start := obj.GetCreationTimestamp()
	conds := objectSet.GetConditions()
	for _, cond := range *conds {
		if cond.Type == "Success" {
			t := cond.LastTransitionTime.Sub(start.Time).Seconds()
			r.objectSetRolloutTime.WithLabelValues(obj.GetName()).Set(t)
		}
	}
}

func (r *Recorder) RecordRolloutTimeObjectSetPhase(objectSetPhase GenericObjectSetPhase) {
	obj := objectSetPhase.ClientObject()
	start := obj.GetCreationTimestamp()
	conds := objectSetPhase.GetConditions()
	for _, cond := range *conds {
		if cond.Type == "Success" {
			t := cond.LastTransitionTime.Sub(start.Time).Seconds()
			r.objectSetPhaseRolloutTime.WithLabelValues(obj.GetName()).Set(t)
		}
	}
}

func (r *Recorder) RecordDynamicCacheSizeGvk(count int) {
	r.dynamicCacheSizeGvk.Set(float64(count))
}

func (r *Recorder) RecordDynamicCacheSizeObj(count int) {
	r.dynamicCacheSizeObjects.Set(float64(count))
}

func (r *Recorder) GetDynamicCacheSizeGvk() prometheus.Gauge {
	return r.dynamicCacheSizeGvk
}

// GetDynamicCacheSizeObj is used for testing Cache.SampleMetrics.
func (r *Recorder) GetDynamicCacheSizeObj() prometheus.Gauge {
	return r.dynamicCacheSizeObjects
}
