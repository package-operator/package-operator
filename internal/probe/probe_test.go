package probe

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"testing"
)

var test = unstructured.Unstructured{
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
			"generation":        int64(1),
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
			"observedGeneration": int64(1),
		},
	},
}

var test2 = unstructured.Unstructured{
	Object: map[string]interface{}{
		"kind":       "test_kind",
		"apiVersion": "test_version",
		"metadata": map[string]interface{}{
			"name":              "test",
			"namespace":         "test",
			"generateName":      "test_generateName",
			"uid":               "test_uid",
			"resourceVersion":   "test_resourceVersion",
			"selfLink":          "test_selfLink",
			"creationTimestamp": "2009-11-10T23:00:00Z",
			"deletionTimestamp": "2010-11-10T23:00:00Z",
			"generation":        int64(1),
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
					"status": "True",
				},
			},
			"observedGeneration": int64(1),
		},
	},
}

var test3 = unstructured.Unstructured{
	Object: map[string]interface{}{
		"kind":       "test_kind",
		"apiVersion": "test_version",
		"metadata": map[string]interface{}{
			"name":              "test",
			"namespace":         "test",
			"generateName":      "test_generateName",
			"uid":               "test_uid",
			"resourceVersion":   "test_resourceVersion",
			"selfLink":          "test_selfLink",
			"creationTimestamp": "2009-11-10T23:00:00Z",
			"deletionTimestamp": "2010-11-10T23:00:00Z",
			"generation":        int64(2),
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
					"status": "True",
				},
			},
			"observedGeneration": int64(1),
		},
	},
}

func TestProbe(t *testing.T) {
	tests := []struct {
		name                  string
		obj                   *unstructured.Unstructured
		passFieldEqual        bool
		passCondition         bool
		passCurrentGeneration bool
	}{
		{
			name:                  "Fields unequal, condition wrong, up to date generation",
			obj:                   &test,
			passFieldEqual:        false,
			passCondition:         false,
			passCurrentGeneration: true,
		},
		{
			name:                  "Fields equal, condition correct, up to date generation",
			obj:                   &test2,
			passFieldEqual:        true,
			passCondition:         true,
			passCurrentGeneration: true,
		},
		{
			name:                  "Fields equal, condition correct, out dated generation",
			obj:                   &test3,
			passFieldEqual:        false,
			passCondition:         true,
			passCurrentGeneration: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cp := ConditionProbe{
				Type:   "Available",
				Status: "True",
			}
			success, _ := cp.Probe(test.obj)
			assert.Equal(t, test.passCondition, success)

			fep := FieldsEqualProbe{
				FieldA: "metadata.name",
				FieldB: "metadata.namespace",
			}
			success, _ = fep.Probe(test.obj)
			assert.Equal(t, test.passFieldEqual, success)

			cgp := CurrentGenerationProbe{}
			success, _ = cgp.Probe(test.obj)
			assert.Equal(t, test.passCurrentGeneration, success)
		})
	}
}
