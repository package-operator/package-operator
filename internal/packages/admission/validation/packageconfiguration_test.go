package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestValidatePackageConfiguration(t *testing.T) {
	tests := []struct {
		name                  string
		packageManifestConfig *manifestsv1alpha1.PackageManifestSpecConfig
		configJSON            string
		expectedErrors        []string
	}{
		{
			name: "wrong type and missing required",
			packageManifestConfig: &manifestsv1alpha1.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"test": {
							Type: "string",
						},
					},
					Required: []string{"test", "banana"},
				},
			},
			configJSON: `{"test":42}`,
			expectedErrors: []string{
				`test: Invalid value: "number": test in body must be of type string: "number"`,
				"banana: Required value",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &runtime.RawExtension{
				Raw: []byte(test.configJSON),
			}

			ctx := context.Background()
			ferrs, err := ValidatePackageConfiguration(ctx, testScheme, test.packageManifestConfig, config, nil)
			require.NoError(t, err)

			var errorStrings []string
			for _, err := range ferrs {
				errorStrings = append(errorStrings, err.Error())
			}
			assert.Len(t, errorStrings, len(test.expectedErrors))
			for _, expectedError := range test.expectedErrors {
				assert.Contains(t, errorStrings, expectedError)
			}
		})
	}
}
