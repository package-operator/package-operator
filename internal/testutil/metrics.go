package testutil

import (
	"bytes"
	"context"
	"strings"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func ParseMetrics(metricBytes []byte) (map[string][]*io_prometheus_client.Metric, error) {
	reader := bytes.NewReader(metricBytes)
	parser := &expfmt.TextParser{}

	mfs, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}

	metrics := make(map[string][]*io_prometheus_client.Metric)
	pfx := strings.ReplaceAll("package-operator", "-", "_") + "_"

	for name, mf := range mfs {
		if strings.HasPrefix(name, pfx) {
			metrics[strings.TrimPrefix(name, pfx)] = mf.GetMetric()
		}
	}
	return metrics, nil
}

func VerifyMetrics(ctx context.Context, metric, label, value string) (bool, error) {
	respBytes, err := GetEndpointOnCluster(ctx, "package-operator-system", "package-operator-metrics", "/metrics", 8080)
	if err != nil {
		return false, err
	}
	metrics, err := ParseMetrics(respBytes)
	if err != nil {
		return false, err
	}

	// Verify there's a vector for the package
	vectors := metrics[metric]
	vector := searchableMetrics(vectors).FindMetric(label, value)
	if vector != nil {
		return true, nil
	}
	return false, nil
}

type searchableMetrics []*io_prometheus_client.Metric

func (ms searchableMetrics) FindMetric(label, value string) *io_prometheus_client.Metric {
	for _, m := range ms {
		labels := searchableLables(m.GetLabel())

		if labels.Contains(label, value) {
			return m
		}
	}
	return nil
}

type searchableLables []*io_prometheus_client.LabelPair

func (ls searchableLables) Contains(name, val string) bool {
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
