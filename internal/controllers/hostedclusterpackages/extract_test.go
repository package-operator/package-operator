package hostedclusterpackages

import (
	"fmt"
	"testing"

	"github.com/erdii/matrix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

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

// TestExtractPackageTemplateFields_MatrixExhaustive
// test all combinations of set/unset fields in HostedClusterPackage.spec.template.
// This ensures that ExtractPackageTemplateFields correctly handles every possible
// field configuration and doesn't apply Go zero-value defaulting.
func TestExtractPackageTemplateFields_MatrixExhaustive(t *testing.T) {
	t.Parallel()

	// Define the test case structure using booleans to indicate whether fields should be set
	// All fields must be exported for the matrix library to work
	// Using pointer for Paused to test nil (unset), false, and true states
	type TestCase struct {
		SetImage       bool
		SetConfig      bool
		SetComponent   bool
		Paused         *bool
		SetLabels      bool
		SetAnnotations bool
	}

	// Generate all combinations using matrix library (v0.0.2+ supports nil pointers)
	// 2*2*2*3*2*2 = 96 test cases covering all possible combinations
	for tc := range matrix.Generate(t, TestCase{},
		[]bool{false, true},                       // SetImage
		[]bool{false, true},                       // SetConfig
		[]bool{false, true},                       // SetComponent
		[]*bool{nil, ptr.To(false), ptr.To(true)}, // Paused: nil=unset, false, true
		[]bool{false, true},                       // SetLabels
		[]bool{false, true},                       // SetAnnotations
	) {
		pausedStr := "unset"
		if tc.Paused != nil {
			if *tc.Paused {
				pausedStr = "true"
			} else {
				pausedStr = "false"
			}
		}

		testName := fmt.Sprintf("image=%v_config=%v_component=%v_paused=%s_labels=%v_annotations=%v",
			tc.SetImage,
			tc.SetConfig,
			tc.SetComponent,
			pausedStr,
			tc.SetLabels,
			tc.SetAnnotations,
		)

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// Build the unstructured HostedClusterPackage with only the fields that are set
			hcpkgObj := map[string]any{
				"apiVersion": "package-operator.run/v1alpha1",
				"kind":       "HostedClusterPackage",
				"metadata": map[string]any{
					"name": "test-hcpkg",
				},
				"spec": map[string]any{
					"template": map[string]any{},
				},
			}

			// Build template.metadata only if labels or annotations are set
			templateMeta := map[string]any{}
			if tc.SetLabels {
				templateMeta["labels"] = map[string]any{"label-key": "label-value"}
			}
			if tc.SetAnnotations {
				templateMeta["annotations"] = map[string]any{"annotation-key": "annotation-val"}
			}
			if len(templateMeta) > 0 {
				hcpkgObj["spec"].(map[string]any)["template"].(map[string]any)["metadata"] = templateMeta
			}

			// Build template.spec with only the fields that are set
			templateSpec := map[string]any{}
			if tc.SetImage {
				templateSpec["image"] = "test-image:v1"
			}
			if tc.SetConfig {
				templateSpec["config"] = map[string]any{"key": "value"}
			}
			if tc.SetComponent {
				templateSpec["component"] = "test-component"
			}
			if tc.Paused != nil {
				templateSpec["paused"] = *tc.Paused
			}

			hcpkgObj["spec"].(map[string]any)["template"].(map[string]any)["spec"] = templateSpec

			// Create unstructured object
			uns := &unstructured.Unstructured{Object: hcpkgObj}

			// Call ExtractPackageTemplateFields
			result, err := ExtractPackageTemplateFields(uns)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.Spec)

			// Validate spec fields
			if tc.SetImage {
				require.NotNil(t, result.Spec.Image, "Image should be set when provided")
				assert.Equal(t, "test-image:v1", *result.Spec.Image)
			} else {
				assert.Nil(t, result.Spec.Image, "Image should be nil when not set")
			}

			if tc.SetConfig {
				require.NotNil(t, result.Spec.Config, "Config should be set when provided")
				assert.NotNil(t, result.Spec.Config.Raw)
			} else {
				assert.Nil(t, result.Spec.Config, "Config should be nil when not set")
			}

			if tc.SetComponent {
				require.NotNil(t, result.Spec.Component, "Component should be set when provided")
				assert.Equal(t, "test-component", *result.Spec.Component)
			} else {
				assert.Nil(t, result.Spec.Component, "Component should be nil when not set")
			}

			if tc.Paused != nil {
				require.NotNil(t, result.Spec.Paused, "Paused should be set when explicitly provided")
				assert.Equal(t, *tc.Paused, *result.Spec.Paused,
					"Paused value should match (critical for SSA - false is different from nil)")
			} else {
				assert.Nil(t, result.Spec.Paused,
					"Paused should be nil when not set (critical for SSA - prevents overriding user intents)")
			}

			// Validate metadata fields
			if tc.SetLabels {
				require.NotNil(t, result.Labels, "Labels should be set when provided")
				assert.Equal(t, map[string]string{"label-key": "label-value"}, result.Labels)
			} else {
				assert.Nil(t, result.Labels, "Labels should be nil when not set")
			}

			if tc.SetAnnotations {
				require.NotNil(t, result.Annotations, "Annotations should be set when provided")
				assert.Equal(t, map[string]string{"annotation-key": "annotation-val"}, result.Annotations)
			} else {
				assert.Nil(t, result.Annotations, "Annotations should be nil when not set")
			}
		})
	}
}
