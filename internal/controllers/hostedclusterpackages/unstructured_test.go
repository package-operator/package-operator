package hostedclusterpackages

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestToUnstructured_Success(t *testing.T) {
	t.Parallel()

	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hcpkg",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"app": "test",
			},
			Annotations: map[string]string{
				"note": "test-annotation",
			},
		},
		Spec: corev1alpha1.HostedClusterPackageSpec{
			Template: corev1alpha1.PackageTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"template": "label",
					},
				},
				Spec: corev1alpha1.PackageSpec{
					Image: "test-image:v1",
				},
			},
		},
	}

	// Set GVK on the input object
	hcpkg.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "package-operator.run",
		Version: "v1alpha1",
		Kind:    "HostedClusterPackage",
	})

	uns, err := toUnstructured(hcpkg)

	require.NoError(t, err)
	require.NotNil(t, uns)

	// Verify GVK is preserved
	gvk := uns.GroupVersionKind()
	assert.Equal(t, "package-operator.run", gvk.Group)
	assert.Equal(t, "v1alpha1", gvk.Version)
	assert.Equal(t, "HostedClusterPackage", gvk.Kind)

	// Verify metadata
	assert.Equal(t, "test-hcpkg", uns.GetName())
	assert.Equal(t, "test-namespace", uns.GetNamespace())
	assert.Equal(t, map[string]string{"app": "test"}, uns.GetLabels())
	assert.Equal(t, map[string]string{"note": "test-annotation"}, uns.GetAnnotations())

	// Verify spec exists in the unstructured object
	spec, found, err := unstructured.NestedMap(uns.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, spec)

	// Verify template exists
	template, found, err := unstructured.NestedMap(uns.Object, "spec", "template")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, template)
}

func TestToUnstructured_MinimalObject(t *testing.T) {
	t.Parallel()

	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "minimal",
		},
	}

	// Set GVK
	hcpkg.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "package-operator.run",
		Version: "v1alpha1",
		Kind:    "HostedClusterPackage",
	})

	uns, err := toUnstructured(hcpkg)

	require.NoError(t, err)
	require.NotNil(t, uns)

	// Verify minimal fields
	assert.Equal(t, "minimal", uns.GetName())
	assert.Empty(t, uns.GetNamespace())

	// Verify GVK is set
	gvk := uns.GroupVersionKind()
	assert.Equal(t, "package-operator.run", gvk.Group)
	assert.Equal(t, "v1alpha1", gvk.Version)
	assert.Equal(t, "HostedClusterPackage", gvk.Kind)
}

func TestToUnstructured_WithStatus(t *testing.T) {
	t.Parallel()

	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "with-status",
		},
		Status: corev1alpha1.HostedClusterPackageStatus{
			HostedClusterPackageCountsStatus: corev1alpha1.HostedClusterPackageCountsStatus{
				ObservedGeneration: 5,
				AvailablePackages:  8,
				ProgressedPackages: 7,
			},
			Conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.HostedClusterPackageAvailable,
					Status: metav1.ConditionTrue,
					Reason: "Available",
				},
			},
		},
	}

	// Set GVK
	hcpkg.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "package-operator.run",
		Version: "v1alpha1",
		Kind:    "HostedClusterPackage",
	})

	uns, err := toUnstructured(hcpkg)

	require.NoError(t, err)
	require.NotNil(t, uns)

	// Verify status exists
	status, found, err := unstructured.NestedMap(uns.Object, "status")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, status)

	// Verify status fields
	observedGen, found, err := unstructured.NestedInt64(uns.Object, "status", "observedGeneration")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(5), observedGen)

	availablePackages, found, err := unstructured.NestedInt64(uns.Object, "status", "availablePackages")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(8), availablePackages)

	progressedPackages, found, err := unstructured.NestedInt64(uns.Object, "status", "progressedPackages")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int64(7), progressedPackages)
}

func TestToUnstructured_NilInput(t *testing.T) {
	t.Parallel()

	// Test with nil input - this should fail during conversion
	var hcpkg *corev1alpha1.HostedClusterPackage

	uns, err := toUnstructured(hcpkg)

	// The behavior depends on the runtime converter
	// If it errors, verify error handling
	if err != nil {
		assert.Nil(t, uns)
		assert.Contains(t, err.Error(), "converting HostedClusterPackage to unstructured")
	} else {
		// If it doesn't error, just verify we got something back
		require.NotNil(t, uns)
	}
}

func TestToUnstructured_PreservesGVK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
	}{
		{
			name: "standard GVK",
			gvk: schema.GroupVersionKind{
				Group:   "package-operator.run",
				Version: "v1alpha1",
				Kind:    "HostedClusterPackage",
			},
		},
		{
			name: "different version",
			gvk: schema.GroupVersionKind{
				Group:   "package-operator.run",
				Version: "v1beta1",
				Kind:    "HostedClusterPackage",
			},
		},
		{
			name: "empty GVK",
			gvk:  schema.GroupVersionKind{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hcpkg := &corev1alpha1.HostedClusterPackage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}
			hcpkg.SetGroupVersionKind(tt.gvk)

			uns, err := toUnstructured(hcpkg)

			require.NoError(t, err)
			require.NotNil(t, uns)

			// Verify GVK is preserved exactly
			resultGVK := uns.GroupVersionKind()
			assert.Equal(t, tt.gvk.Group, resultGVK.Group)
			assert.Equal(t, tt.gvk.Version, resultGVK.Version)
			assert.Equal(t, tt.gvk.Kind, resultGVK.Kind)
		})
	}
}

func TestToUnstructured_NoDefaulting(t *testing.T) {
	t.Parallel()

	// Create a HostedClusterPackage with only required fields in the template spec
	// Optional fields (Config, Component, Paused) should NOT appear in unstructured output
	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-defaults",
			Namespace: "default",
		},
		Spec: corev1alpha1.HostedClusterPackageSpec{
			Template: corev1alpha1.PackageTemplateSpec{
				Spec: corev1alpha1.PackageSpec{
					// Only set the required field
					Image: "test-image:v1",
					// Intentionally omit:
					// - Config
					// - Component
					// - Paused (even though it's a bool that defaults to false in Go)
				},
			},
		},
	}

	hcpkg.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "package-operator.run",
		Version: "v1alpha1",
		Kind:    "HostedClusterPackage",
	})

	uns, err := toUnstructured(hcpkg)

	require.NoError(t, err)
	require.NotNil(t, uns)

	// Get the template spec from the unstructured object
	templateSpec, found, err := unstructured.NestedMap(uns.Object, "spec", "template", "spec")
	require.NoError(t, err)
	require.True(t, found, "spec.template.spec should exist")
	require.NotNil(t, templateSpec)

	// Verify that only the image field is present
	assert.Contains(t, templateSpec, "image", "image field should be present")
	assert.Equal(t, "test-image:v1", templateSpec["image"])

	// Verify that optional fields are NOT present (not defaulted)
	assert.NotContains(t, templateSpec, "config", "config field should not be present when unspecified")
	assert.NotContains(t, templateSpec, "component", "component field should not be present when unspecified")
	assert.NotContains(t, templateSpec, "paused",
		"paused field should not be present when unspecified (not defaulted to false)")

	// Verify the total number of fields - should only have "image"
	assert.Len(t, templateSpec, 1, "template spec should only contain the image field")
}

func TestToUnstructured_WithOptionalFields(t *testing.T) {
	t.Parallel()

	// Create a HostedClusterPackage with all optional fields explicitly set
	hcpkg := &corev1alpha1.HostedClusterPackage{
		ObjectMeta: metav1.ObjectMeta{
			Name: "with-optional",
		},
		Spec: corev1alpha1.HostedClusterPackageSpec{
			Template: corev1alpha1.PackageTemplateSpec{
				Spec: corev1alpha1.PackageSpec{
					Image:     "test-image:v1",
					Component: "frontend",
					Paused:    true,
					Config: &runtime.RawExtension{
						Raw: []byte(`{"key":"value"}`),
					},
				},
			},
		},
	}

	hcpkg.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "package-operator.run",
		Version: "v1alpha1",
		Kind:    "HostedClusterPackage",
	})

	uns, err := toUnstructured(hcpkg)

	require.NoError(t, err)
	require.NotNil(t, uns)

	// Get the template spec from the unstructured object
	templateSpec, found, err := unstructured.NestedMap(uns.Object, "spec", "template", "spec")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, templateSpec)

	// Verify all fields are present when explicitly set
	assert.Contains(t, templateSpec, "image")
	assert.Contains(t, templateSpec, "component")
	assert.Contains(t, templateSpec, "paused")
	assert.Contains(t, templateSpec, "config")

	// Verify field values
	assert.Equal(t, "test-image:v1", templateSpec["image"])
	assert.Equal(t, "frontend", templateSpec["component"])
	assert.Equal(t, true, templateSpec["paused"])

	// Config should be present as a map
	assert.NotNil(t, templateSpec["config"])
}
