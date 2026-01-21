package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// Recorder stores all the metrics related to Addons.
type Recorder struct {
	packageAvailability *prometheus.GaugeVec
	packageCreated      *prometheus.GaugeVec
	packageLoadDuration *prometheus.GaugeVec
	packageRevision     *prometheus.GaugeVec

	objectSetCreated   *prometheus.GaugeVec
	objectSetSucceeded *prometheus.GaugeVec
}

func NewRecorder() *Recorder {
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
		}, []string{"pko_name", "pko_namespace", "pko_package_instance", "image"},
	)

	return &Recorder{
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
		r.packageAvailability, r.packageCreated, r.packageLoadDuration, r.packageRevision,

		r.objectSetCreated, r.objectSetSucceeded,
	)
}

type GenericPackage interface {
	ClientObject() client.Object
	GetSpecImage() string
	GetStatusConditions() *[]metav1.Condition
	GetStatusRevision() int64
}

func (r *Recorder) RecordPackageMetrics(pkg GenericPackage) {
	obj := pkg.ClientObject()
	if !obj.GetDeletionTimestamp().IsZero() {
		r.packageAvailability.DeletePartialMatch(prometheus.Labels{
			"pko_name":      obj.GetName(),
			"pko_namespace": obj.GetNamespace(),
		})
		r.packageCreated.DeleteLabelValues(obj.GetName(), obj.GetNamespace())
		r.packageLoadDuration.DeleteLabelValues(obj.GetName(), obj.GetNamespace())
		r.packageRevision.DeleteLabelValues(obj.GetName(), obj.GetNamespace())
		return
	}

	// default to unknown
	healthStatus := 2

	if availableCond := meta.FindStatusCondition(
		*pkg.GetStatusConditions(), corev1alpha1.PackageAvailable,
	); availableCond != nil {
		switch availableCond.Status {
		case metav1.ConditionFalse:
			healthStatus = 0
		case metav1.ConditionTrue:
			healthStatus = 1
		}
	}

	// Delete all old availability timeseries for this package first.
	// This is needed because changes to the image label will introduce a new timeseries
	// and we only want 1 timeseries for any given package.
	// This is racy, because scraping could happen in between and result in 0 timeseries.
	// OLM does the same though:
	// https://github.com/operator-framework/operator-lifecycle-manager/blob/abd99636e779f0bfbce31225e377d6bfd4fa3b9b/pkg/metrics/metrics.go#L302-L324
	r.packageAvailability.DeletePartialMatch(prometheus.Labels{
		"pko_name":      obj.GetName(),
		"pko_namespace": obj.GetNamespace(),
	})
	r.packageAvailability.WithLabelValues(
		obj.GetName(), obj.GetNamespace(), pkg.GetSpecImage(),
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
	GetStatusConditions() *[]metav1.Condition
}

func (r *Recorder) RecordObjectSetMetrics(objectSet GenericObjectSet) {
	obj := objectSet.ClientObject()

	// Package instance name -> name of the Package Object.
	var instance string
	if l := obj.GetLabels(); l != nil && l[manifestsv1alpha1.PackageInstanceLabel] != "" {
		instance = l[manifestsv1alpha1.PackageInstanceLabel]
	}

	// Package source image -> image of the Package Object.
	var image string
	annotations := obj.GetAnnotations()
	if annotations != nil && annotations[manifestsv1alpha1.PackageSourceImageAnnotation] != "" {
		image = annotations[manifestsv1alpha1.PackageSourceImageAnnotation]
	}

	if !obj.GetDeletionTimestamp().IsZero() ||
		meta.IsStatusConditionTrue(*objectSet.GetStatusConditions(), corev1alpha1.ObjectSetArchived) {
		r.objectSetSucceeded.DeleteLabelValues(obj.GetName(), obj.GetNamespace(), instance, image)
	} else {
		succeededCond := meta.FindStatusCondition(*objectSet.GetStatusConditions(), corev1alpha1.ObjectSetSucceeded)
		if succeededCond != nil {
			r.objectSetSucceeded.
				WithLabelValues(obj.GetName(), obj.GetNamespace(), instance, image).
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
