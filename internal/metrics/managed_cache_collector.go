package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"pkg.package-operator.run/boxcutter/managedcache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ ManagedCacheCollector = (*collector)(nil)

// ManagedCacheCollector is an alias for prometheus.Collector.
type ManagedCacheCollector prometheus.Collector

// NewManagedCacheCollector constructs a managed cache metrics collector that collects metrics from the provided ObjectBoundAccessManager.
func NewManagedCacheCollector(manager managedcache.ObjectBoundAccessManager[client.Object], log logr.Logger) ManagedCacheCollector {
	informersDesc := prometheus.NewDesc(
		"package_operator_managed_cache_informers_total",
		"Number of active informers per owner running for the managed cache.",
		[]string{"owner"}, nil)
	objectsDesc := prometheus.NewDesc(
		"package_operator_managed_cache_objects_total",
		"Number of objects per GVK and owner in the managed cache.",
		[]string{"owner", "gvk"}, nil)

	return &collector{
		manager,
		informersDesc,
		objectsDesc,
		log,
	}
}

type collector struct {
	manager       managedcache.ObjectBoundAccessManager[client.Object]
	informersDesc *prometheus.Desc
	objectsDesc   *prometheus.Desc
	log           logr.Logger
}

func (c collector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c collector) Collect(ch chan<- prometheus.Metric) {
	objectsPerOwnerPerGVK, err := c.manager.CollectMetrics(context.Background())

	if err != nil {
		c.log.Error(err, "collecting managed cache metrics")
	}

	for owner, objectsPerGVK := range objectsPerOwnerPerGVK {
		// Number of GVKs per owner
		ch <- prometheus.MustNewConstMetric(
			c.informersDesc,
			prometheus.GaugeValue,
			float64(len(objectsPerGVK)),
			string(owner.UID),
		)

		for gvk, objects := range objectsPerGVK {
			// Number of objects per GVK per owner
			ch <- prometheus.MustNewConstMetric(
				c.objectsDesc,
				prometheus.GaugeValue,
				float64(objects),
				string(owner.UID), gvk.String(),
			)
		}
	}
}
