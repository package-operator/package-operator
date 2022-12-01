package packagestructure

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

	pkoapis "package-operator.run/apis"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/packagebytes"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestLoader(t *testing.T) {
	l := NewLoader(testScheme)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	pc, err := l.LoadFromPath(ctx, "testdata", WithByteTransformers(
		&packagebytes.TemplateTransformer{
			TemplateContext: manifestsv1alpha1.TemplateContext{
				Package: manifestsv1alpha1.TemplateContextPackage{
					TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{
						Namespace: "test123-ns",
					},
				},
			},
		}))
	require.NoError(t, err)

	expectedProbes := []corev1alpha1.ObjectSetProbe{
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
	}

	assert.Equal(t, &manifestsv1alpha1.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-package",
		},
		Spec: manifestsv1alpha1.PackageManifestSpec{
			Scopes: []manifestsv1alpha1.PackageManifestScope{
				manifestsv1alpha1.PackageManifestScopeNamespaced,
			},
			Phases: []manifestsv1alpha1.PackageManifestPhase{
				{Name: "pre-requisites"},
				{Name: "main-stuff"},
				{Name: "empty"},
			},
			AvailabilityProbes: expectedProbes,
		},
	}, pc.PackageManifest)

	spec := pc.ToTemplateSpec()
	assert.Equal(t, expectedProbes, spec.AvailabilityProbes)
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
								"name": "some-configmap",
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
								"name": "some-service-account",
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
								"name":      "controller-manager",
								"namespace": "test123-ns",
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
								"name": "some-stateful-set-1",
							},
							"spec": map[string]interface{}{},
						},
					},
				},
			},
		},
	}, spec.Phases)
}

func TestLoaderOptions(t *testing.T) {
	opts := LoaderOptions{
		bytesTransformers: packagebytes.TransformerList{
			packagebytes.TransformerList{},
		},
		bytesValidators: packagebytes.ValidatorList{
			packagebytes.ValidatorList{},
		},
		manifestTransformers: TransformerList{
			TransformerList{},
		},
		manifestValidators: ValidatorList{
			ValidatorList{},
		},
	}
	WithByteTransformers(packagebytes.TransformerList{})(&opts)
	assert.Len(t, opts.bytesTransformers, 2)

	WithByteValidators(packagebytes.ValidatorList{})(&opts)
	assert.Len(t, opts.bytesValidators, 2)

	WithManifestTransformers(TransformerList{})(&opts)
	assert.Len(t, opts.bytesTransformers, 2)

	WithManifestValidators(ValidatorList{})(&opts)
	assert.Len(t, opts.bytesValidators, 2)
}
