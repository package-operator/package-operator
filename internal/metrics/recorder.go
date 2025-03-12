package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// Recorder stores all the metrics related to Addons.
type Recorder struct {
	dynamicCacheInformers prometheus.Gauge
	dynamicCacheObjects   *prometheus.GaugeVec

	packageAvailability *prometheus.GaugeVec
	packageCreated      *prometheus.GaugeVec
	packageLoadDuration *prometheus.GaugeVec
	packageRevision     *prometheus.GaugeVec

	objectSetCreated   *prometheus.GaugeVec
	objectSetSucceeded *prometheus.GaugeVec
}

func NewRecorder() *Recorder {
	// DynamicCache
	dynamicCacheInformers := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_informers",
			Help: "Tracks the number of active Informers running for the dynamic cache.",
		})
	dynamicCacheObjects := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_dynamic_cache_objects",
			Help: "Number of objects for each GVK in the dynamic cache.",
		}, []string{"pko_gvk"})

	// Package
	packageAvailability := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_package_availability",
			Help: "Package availability 0=Unavailable,1=Available,2=Unknown.",
		}, []string{"pko_name", "pko_namespace", "image"},
	)
	packageCreated := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_package_created_timestamp_seconds",
			Help: "Package Unix creation timestamp.",
		}, []string{"pko_name", "pko_namespace"},
	)
	packageLoadDuration := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_package_load_duration_seconds",
			Help: "Duration for the last package load operation, including download and parsing.",
		}, []string{"pko_name", "pko_namespace"},
	)
	packageRevision := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_package_revision",
			Help: "Package revision.",
		}, []string{"pko_name", "pko_namespace"},
	)

	// Revisions
	objectSetCreated := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_object_set_created_timestamp_seconds",
			Help: "ObjectSet Unix creation timestamp.",
		}, []string{"pko_name", "pko_namespace", "pko_package_instance"},
	)
	objectSetSucceeded := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "package_operator_object_set_succeeded_timestamp_seconds",
			Help: "ObjectSet Unix success timestamp.",
		}, []string{"pko_name", "pko_namespace", "pko_package_instance"},
	)

	return &Recorder{
		dynamicCacheInformers: dynamicCacheInformers,
		dynamicCacheObjects:   dynamicCacheObjects,

		packageAvailability: packageAvailability,
		packageCreated:      packageCreated,
		packageLoadDuration: packageLoadDuration,
		packageRevision:     packageRevision,

		objectSetCreated:   objectSetCreated,
		objectSetSucceeded: objectSetSucceeded,
	}
}

// Register metrics into ctrl registry.
func (r *Recorder) Register() {
	metrics.Registry.MustRegister(
		r.dynamicCacheInformers, r.dynamicCacheObjects,
		r.packageAvailability, r.packageCreated, r.packageLoadDuration, r.packageRevision,

		r.objectSetCreated, r.objectSetSucceeded,
	)
}

type GenericPackage interface {
	ClientObject() client.Object
	GetImage() string
	GetConditions() *[]metav1.Condition
	GetStatusRevision() int64
}

func (r *Recorder) RecordPackageMetrics(pkg GenericPackage) {
	obj := pkg.ClientObject()
	if !obj.GetDeletionTimestamp().IsZero() {
		r.packageAvailability.DeleteLabelValues(obj.GetName(), obj.GetNamespace())
		r.packageCreated.DeleteLabelValues(obj.GetName(), obj.GetNamespace())
		r.packageLoadDuration.DeleteLabelValues(obj.GetName(), obj.GetNamespace())
		r.packageRevision.DeleteLabelValues(obj.GetName(), obj.GetNamespace())
		return
	}

	// default to unknown
	healthStatus := 2

	if availableCond := meta.FindStatusCondition(
		*pkg.GetConditions(), corev1alpha1.PackageAvailable,
	); availableCond != nil {
		switch availableCond.Status {
		case metav1.ConditionFalse:
			healthStatus = 0
		case metav1.ConditionTrue:
			healthStatus = 1
		}
	}

	r.packageAvailability.WithLabelValues(
		obj.GetName(), obj.GetNamespace(), pkg.GetImage(),
	).Set(float64(healthStatus))
	r.packageCreated.WithLabelValues(
		obj.GetName(), obj.GetNamespace(),
	).Set(float64(obj.GetCreationTimestamp().Unix()))
	r.packageRevision.WithLabelValues(
		obj.GetName(), obj.GetNamespace(),
	).Set(float64(pkg.GetStatusRevision()))
}

func (r *Recorder) RecordPackageLoadMetric(pkg GenericPackage, d time.Duration) {
	obj := pkg.ClientObject()
	r.packageLoadDuration.
		WithLabelValues(obj.GetName(), obj.GetNamespace()).
		Set(float64(d.Milliseconds()) / 1000)
}

type GenericObjectSet interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
}

func (r *Recorder) RecordObjectSetMetrics(objectSet GenericObjectSet) {
	obj := objectSet.ClientObject()

	// Package instance name -> name of the Package Object.
	var instance string
	if l := obj.GetLabels(); l != nil && l[manifestsv1alpha1.PackageInstanceLabel] != "" {
		instance = l[manifestsv1alpha1.PackageInstanceLabel]
	}

	if !obj.GetDeletionTimestamp().IsZero() ||
		meta.IsStatusConditionTrue(*objectSet.GetConditions(), corev1alpha1.ObjectSetArchived) {
		r.objectSetSucceeded.DeleteLabelValues(obj.GetName(), obj.GetNamespace(), instance)
	} else {
		succeededCond := meta.FindStatusCondition(*objectSet.GetConditions(), corev1alpha1.ObjectSetSucceeded)
		if succeededCond != nil {
			r.objectSetSucceeded.
				WithLabelValues(obj.GetName(), obj.GetNamespace(), instance).
				Set(float64(succeededCond.LastTransitionTime.Unix()))
		}
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		r.objectSetCreated.DeleteLabelValues(obj.GetName(), obj.GetNamespace(), instance)
	} else {
		r.objectSetCreated.
			WithLabelValues(obj.GetName(), obj.GetNamespace(), instance).
			Set(float64(obj.GetCreationTimestamp().Unix()))
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
