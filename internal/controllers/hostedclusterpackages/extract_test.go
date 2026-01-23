package hostedclusterpackages

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1alpha1acs "package-operator.run/apis/applyconfigurations/core/v1alpha1"
)

//nolint:maintidx // Table-driven test with comprehensive test cases
func TestExtractPackageTemplateFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		hcpkg         *unstructured.Unstructured
		expectError   bool
		errorContains string
		validate      func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration)
	}{
		{
			name: "extracts all spec fields",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name":      "test-package",
								"namespace": "test-ns",
								"labels": map[string]interface{}{
									"app": "test",
								},
								"annotations": map[string]interface{}{
									"key": "value",
								},
							},
							"spec": map[string]interface{}{
								"image":     "test-image:v1",
								"component": "test-component",
								"paused":    true,
								"config": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.ObjectMetaApplyConfiguration)
				require.NotNil(t, result.Spec)

				// Validate metadata
				assert.NotNil(t, result.Name)
				assert.Equal(t, "test-package", *result.Name)
				assert.NotNil(t, result.Namespace)
				assert.Equal(t, "test-ns", *result.Namespace)
				assert.Equal(t, map[string]string{"app": "test"}, result.Labels)
				assert.Equal(t, map[string]string{"key": "value"}, result.Annotations)

				// Validate spec
				assert.NotNil(t, result.Spec.Image)
				assert.Equal(t, "test-image:v1", *result.Spec.Image)
				assert.NotNil(t, result.Spec.Component)
				assert.Equal(t, "test-component", *result.Spec.Component)
				assert.NotNil(t, result.Spec.Paused)
				assert.True(t, *result.Spec.Paused)
				assert.NotNil(t, result.Spec.Config)
			},
		},
		{
			name: "unspecified spec fields are nil - no image",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"component": "test-component",
								"paused":    true,
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.Spec)

				// Image should be nil when not specified
				assert.Nil(t, result.Spec.Image)
				// Component and Paused should be set
				assert.NotNil(t, result.Spec.Component)
				assert.Equal(t, "test-component", *result.Spec.Component)
				assert.NotNil(t, result.Spec.Paused)
				assert.True(t, *result.Spec.Paused)
				// Config should be nil when not specified
				assert.Nil(t, result.Spec.Config)
			},
		},
		{
			name: "unspecified spec fields are nil - no component",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"image":  "test-image:v1",
								"paused": false,
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.Spec)

				assert.NotNil(t, result.Spec.Image)
				assert.Equal(t, "test-image:v1", *result.Spec.Image)
				// Component should be nil when not specified
				assert.Nil(t, result.Spec.Component)
				assert.NotNil(t, result.Spec.Paused)
				assert.False(t, *result.Spec.Paused)
				assert.Nil(t, result.Spec.Config)
			},
		},
		{
			name: "unspecified spec fields are nil - no paused",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"image":     "test-image:v1",
								"component": "test-component",
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.Spec)

				assert.NotNil(t, result.Spec.Image)
				assert.Equal(t, "test-image:v1", *result.Spec.Image)
				assert.NotNil(t, result.Spec.Component)
				assert.Equal(t, "test-component", *result.Spec.Component)
				// Paused should be nil when not specified - this is critical for SSA
				assert.Nil(t, result.Spec.Paused)
				assert.Nil(t, result.Spec.Config)
			},
		},
		{
			name: "unspecified spec fields are nil - no config",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"image":     "test-image:v1",
								"component": "test-component",
								"paused":    true,
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.Spec)

				assert.NotNil(t, result.Spec.Image)
				assert.NotNil(t, result.Spec.Component)
				assert.NotNil(t, result.Spec.Paused)
				// Config should be nil when not specified
				assert.Nil(t, result.Spec.Config)
			},
		},
		{
			name: "unspecified metadata fields are nil - no labels",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "test-package",
								"annotations": map[string]interface{}{
									"key": "value",
								},
							},
							"spec": map[string]interface{}{
								"image": "test-image:v1",
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.ObjectMetaApplyConfiguration)

				assert.NotNil(t, result.Name)
				assert.Equal(t, "test-package", *result.Name)
				// Labels should be nil when not specified
				assert.Nil(t, result.Labels)
				assert.NotNil(t, result.Annotations)
			},
		},
		{
			name: "unspecified metadata fields are nil - no annotations",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "test-package",
								"labels": map[string]interface{}{
									"app": "test",
								},
							},
							"spec": map[string]interface{}{
								"image": "test-image:v1",
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.ObjectMetaApplyConfiguration)

				assert.NotNil(t, result.Name)
				assert.NotNil(t, result.Labels)
				// Annotations should be nil when not specified
				assert.Nil(t, result.Annotations)
			},
		},
		{
			name: "unspecified metadata fields are nil - no namespace",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "test-package",
							},
							"spec": map[string]interface{}{
								"image": "test-image:v1",
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.ObjectMetaApplyConfiguration)

				assert.NotNil(t, result.Name)
				// Namespace should be nil when not specified
				assert.Nil(t, result.Namespace)
			},
		},
		{
			name: "empty template spec",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.Spec)

				// All fields should be nil when spec is empty
				assert.Nil(t, result.Spec.Image)
				assert.Nil(t, result.Spec.Component)
				assert.Nil(t, result.Spec.Paused)
				assert.Nil(t, result.Spec.Config)
			},
		},
		{
			name: "empty template metadata",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{},
							"spec": map[string]interface{}{
								"image": "test-image:v1",
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.ObjectMetaApplyConfiguration)

				// All metadata fields should be nil when metadata is empty
				assert.Nil(t, result.Name)
				assert.Nil(t, result.Namespace)
				assert.Nil(t, result.Labels)
				assert.Nil(t, result.Annotations)
			},
		},
		{
			name: "missing template",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.ObjectMetaApplyConfiguration)
				require.NotNil(t, result.Spec)

				// Should handle missing template gracefully
				assert.Nil(t, result.Name)
				assert.Nil(t, result.Spec.Image)
			},
		},
		{
			name: "paused explicitly set to false",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"image":  "test-image:v1",
								"paused": false,
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.Spec)

				// When explicitly set to false, it should be present
				assert.NotNil(t, result.Spec.Paused)
				assert.False(t, *result.Spec.Paused)
			},
		},
		{
			name: "config with complex nested structure",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"image": "test-image:v1",
								"config": map[string]interface{}{
									"nested": map[string]interface{}{
										"key": "value",
										"array": []interface{}{
											"item1",
											"item2",
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, ac *corev1alpha1acs.PackageTemplateSpecApplyConfiguration) {
				t.Helper()
				result := ac
				require.NotNil(t, result)
				require.NotNil(t, result.Spec)
				require.NotNil(t, result.Spec.Config)

				// Verify config is properly marshaled
				assert.NotNil(t, result.Spec.Config.Raw)
			},
		},
		{
			name: "invalid metadata structure",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": "invalid-not-a-map",
							"spec": map[string]interface{}{
								"image": "test-image:v1",
							},
						},
					},
				},
			},
			expectError:   true,
			errorContains: "extracting template metadata",
		},
		{
			name: "invalid spec structure",
			hcpkg: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": "invalid-not-a-map",
						},
					},
				},
			},
			expectError:   true,
			errorContains: "extracting template spec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ac, err := ExtractPackageTemplateFields(tt.hcpkg)

			if tt.expectError {
				require.Error(t, err, "expected an error but got none")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, ac)
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
				require.NotNil(t, ac)
				if tt.validate != nil {
					tt.validate(t, ac)
				}
			}
		})
	}
}

// TestExtractPackageTemplateFields_EnsuresNoDefaulting validates that the extraction
// doesn't apply Go zero-value defaulting which would interfere with Server-Side Apply.
// This is critical for fields like .spec.paused where the absence of the field
// has different semantics than an explicit false value.
func TestExtractPackageTemplateFields_EnsuresNoDefaulting(t *testing.T) {
	t.Parallel()

	hcpkg := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test-package",
					},
					"spec": map[string]interface{}{
						"image": "test-image:v1",
						// Intentionally omitting: component, paused, config
					},
				},
			},
		},
	}

	ac, err := ExtractPackageTemplateFields(hcpkg)
	require.NoError(t, err)
	require.NotNil(t, ac)
	require.NotNil(t, ac.Spec)

	// The critical assertion: unspecified fields must be nil pointers,
	// not pointers to zero values. This ensures SSA doesn't override
	// user-specified values in other intents.
	assert.Nil(t, ac.Spec.Component, "component must be nil when unspecified")
	assert.Nil(t, ac.Spec.Paused, "paused must be nil when unspecified, not false")
	assert.Nil(t, ac.Spec.Config, "config must be nil when unspecified")

	// Verify that explicitly specified fields are set
	assert.NotNil(t, ac.Spec.Image)
	assert.Equal(t, "test-image:v1", *ac.Spec.Image)
}
