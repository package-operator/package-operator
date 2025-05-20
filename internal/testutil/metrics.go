package testutil

import (
	"bytes"
	"context"
	"strings"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"k8s.io/client-go/rest"
)

func parseMetrics(metricBytes []byte) (map[string][]*io_prometheus_client.Metric, error) {
	reader := bytes.NewReader(metricBytes)
	parser := &expfmt.TextParser{}

	mfs, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}

	metrics := make(map[string][]*io_prometheus_client.Metric)
	pfx := "package_operator_"

	for name, mf := range mfs {
		if strings.HasPrefix(name, pfx) {
			metrics[strings.TrimPrefix(name, pfx)] = mf.GetMetric()
		}
	}
	return metrics, nil
}

// This function fetches metrics from pko manager and checks if a given vector exists.
func MetricsVectorExists(ctx context.Context, restConfig *rest.Config, metric, label, value string) (bool, error) {
	respBytes, err := GetEndpointOnCluster(ctx, restConfig,
		"package-operator-system", "package-operator-metrics", "/metrics", 8080)
	if err != nil {
		return false, err
	}
	metrics, err := parseMetrics(respBytes)
	if err != nil {
		return false, err
	}

	// Verify there's a vector with the provided label,value pair
	vectors := metrics[metric]
	vector := searchableMetrics(vectors).findMetric(label, value)
	if vector != nil {
		return true, nil
	}
	return false, nil
}

type searchableMetrics []*io_prometheus_client.Metric

func (ms searchableMetrics) findMetric(label, value string) *io_prometheus_client.Metric {
	for _, m := range ms {
		labels := searchableLables(m.GetLabel())

		if labels.contains(label, value) {
			return m
		}
	}
	return nil
}

type searchableLables []*io_prometheus_client.LabelPair

func (ls searchableLables) contains(name, val string) bool {
	for _, l := range ls {
		if l.GetName() != name {
			continue
		}

		if l.GetValue() == val {
			return true
		}
	}
	return false
}
