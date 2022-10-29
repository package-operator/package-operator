package packages

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := manifestsv1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestLoader(t *testing.T) {
	l := NewFolderLoader(testScheme)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	res, err := l.Load(ctx, "./testdata", FolderLoaderTemplateContext{
		Package: PackageTemplateContext{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pack-1",
				Namespace: "test123-ns",
			},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]string{}, res.Annotations)
	assert.Equal(t, map[string]string{
		manifestsv1alpha1.PackageInstanceLabel: "pack-1",
		manifestsv1alpha1.PackageLabel:         "cool-package",
	}, res.Labels)
	assert.Equal(t, []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Group: "apps",
					Kind:  "Deployment",
				},
			},
			Probes: []corev1alpha1.Probe{
				{
					Condition: &corev1alpha1.ProbeConditionSpec{
						Type:   "Available",
						Status: "True",
					},
				},
				{
					FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
						FieldA: ".status.updatedReplicas",
						FieldB: ".status.replicas",
					},
				},
			},
		},
	}, res.TemplateSpec.AvailabilityProbes)

	commonLabels := map[string]interface{}{
		manifestsv1alpha1.PackageLabel:         "cool-package",
		manifestsv1alpha1.PackageInstanceLabel: "pack-1",
	}

	assert.Equal(t, []corev1alpha1.ObjectSetTemplatePhase{
		{
			Name: "pre-requisites",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"annotations": map[string]interface{}{},
								"labels":      commonLabels,
								"name":        "some-configmap",
							},
							"data": map[string]interface{}{
								"foo":   "bar",
								"hello": "world",
							},
						},
					},
				},
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ServiceAccount",
							"metadata": map[string]interface{}{
								"annotations": map[string]interface{}{},
								"labels":      commonLabels,
								"name":        "some-service-account",
							},
						},
					},
				},
			},
		},
		{
			Name: "main-stuff",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]interface{}{
								"annotations": map[string]interface{}{},
								"labels":      commonLabels,
								"name":        "controller-manager",
								"namespace":   "test123-ns",
							},
							"spec": map[string]interface{}{
								"replicas": int64(1),
							},
						},
					},
				},
				{
					Object: unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "StatefulSet",
							"metadata": map[string]interface{}{
								"annotations": map[string]interface{}{},
								"labels":      commonLabels,
								"name":        "some-stateful-set-1",
							},
							"spec": map[string]interface{}{},
						},
					},
				},
			},
		},
	}, res.TemplateSpec.Phases)
}
