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
	mc := &manifestsv1alpha1.PackageManifestSpecConfig{
		OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"test": {
					Type: "string",
				},
			},
			Required: []string{"test", "banana"},
		},
	}
	config := &runtime.RawExtension{
		Raw: []byte(`{"test":123}`),
	}

	ctx := context.Background()
	ferrs, err := ValidatePackageConfiguration(ctx, testScheme, mc, config, nil)
	require.NoError(t, err)

	var errorStrings []string
	for _, err := range ferrs {
		errorStrings = append(errorStrings, err.Error())
	}
	expectedErrors := []string{
		`test: Invalid value: "number": test in body must be of type string: "number"`,
		"banana: Required value",
	}
	assert.Len(t, errorStrings, len(expectedErrors))
	for _, expectedError := range expectedErrors {
		assert.Contains(t, errorStrings, expectedError)
	}

	// assert.Equal(t, `test: Invalid value: "number": test in body must be of type string: "number", banana: Required value`, ferr.ToAggregate().Error())
}
