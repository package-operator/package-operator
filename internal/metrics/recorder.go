package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Recorder stores all the metrics related to Addons.
type Recorder struct {
	dynamicCacheInformers prometheus.Gauge
	dynamicCacheObjects   *prometheus.GaugeVec
	rolloutTime           *prometheus.GaugeVec
}

type GenericObjectSet interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
}

func NewRecorder() *Recorder {
	dynamicCacheInformers := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_informers",
			Help: "Tracks the number of active Informers running for the dynamic cache.",
		})
	dynamicCacheObjects := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_objects",
			Help: "Number of objects for each GVK in the dynamic cache.",
		}, []string{"gvk"})
	rolloutTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_object_set_rollout_time_seconds",
			Help: "Time between Object creationTimestamp and the transition to True of the Succeeded condition.",
		}, []string{"name", "namespace"},
	)

	return &Recorder{
		dynamicCacheInformers: dynamicCacheInformers,
		dynamicCacheObjects:   dynamicCacheObjects,
		rolloutTime:           rolloutTime,
	}
}

// Register metrics into ctrl registry.
func (r *Recorder) Register() {
	ctrlmetrics.Registry.MustRegister(
		r.dynamicCacheInformers,
		r.dynamicCacheObjects,
		r.rolloutTime,
	)
}

func (r *Recorder) RecordRolloutTime(objectSet GenericObjectSet) {
	obj := objectSet.ClientObject()
	start := obj.GetCreationTimestamp()
	conds := objectSet.GetConditions()
	for _, cond := range *conds {
		if cond.Type == corev1alpha1.ObjectSetSucceeded {
			t := cond.LastTransitionTime.Sub(start.Time).Seconds()
			r.rolloutTime.WithLabelValues(obj.GetName(), obj.GetNamespace()).Set(t)
		}
	}
}

// Records the number of active Informers for the cache.
func (r *Recorder) RecordDynamicCacheInformers(total int) {
	r.dynamicCacheInformers.Set(float64(total))
}

// Records the number of objects in the cache identified by GVK.
func (r *Recorder) RecordDynamicCacheObjects(gvk schema.GroupVersionKind, count int) {
	r.dynamicCacheObjects.WithLabelValues(gvk.String()).Set(float64(count))
}
