package probe

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"testing"
)

func TestProbe(t *testing.T) {
	want := unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       "test_kind",
			"apiVersion": "test_version",
			"metadata": map[string]interface{}{
				"name":              "test_name",
				"namespace":         "test_namespace",
				"generateName":      "test_generateName",
				"uid":               "test_uid",
				"resourceVersion":   "test_resourceVersion",
				"selfLink":          "test_selfLink",
				"creationTimestamp": "2009-11-10T23:00:00Z",
				"deletionTimestamp": "2010-11-10T23:00:00Z",
				"labels": map[string]interface{}{
					"test_label": "test_value",
				},
				"annotations": map[string]interface{}{
					"test_annotation": "test_value",
				},
				"ownerReferences": []map[string]interface{}{
					{
						"kind":       "Pod",
						"name":       "poda",
						"apiVersion": "v1",
						"uid":        "1",
						"controller": (*bool)(nil),
					},
					{
						"kind":       "Pod",
						"name":       "podb",
						"apiVersion": "v1",
						"uid":        "2",
						"controller": pointer.Bool(true),
					},
				},
				"finalizers": []interface{}{
					"finalizer.1",
					"finalizer.2",
				},
				"clusterName": "cluster123",
			},
			"status": map[string]interface{}{ // TODO: is this right?
				"conditions": []map[string]interface{}{
					{
						"type":   "Available",
						"status": "False",
					},
				},
			},
		},
	}
}
