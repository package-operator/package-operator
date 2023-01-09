package admission

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := manifestsv1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := apiextensions.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestPackageAdmissionController(t *testing.T) {
	tests := []struct {
		name                  string
		manifest              *manifestsv1alpha1.PackageManifest
		configuration         map[string]interface{}
		expectedErrors        []string
		expectedConfiguration map[string]interface{}
	}{
		{
			name: "wrong type and missing required",
			manifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Config: manifestsv1alpha1.PackageManifestSpecConfig{
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
				},
			},
			configuration: map[string]interface{}{"test": 42.0},
			expectedErrors: []string{
				`test: Invalid value: "number": test in body must be of type string: "number"`,
				"banana: Required value",
			},
		},
		{
			name: "required but default",
			manifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Config: manifestsv1alpha1.PackageManifestSpecConfig{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"test": {
									Type:    "string",
									Default: &apiextensionsv1.JSON{Raw: []byte(`"default_value"`)},
								},
							},
							Required: []string{"test"},
						},
					},
				},
			},
			configuration:         map[string]interface{}{},
			expectedConfiguration: map[string]interface{}{"test": "default_value"},
			expectedErrors:        []string{},
		},
		{
			name: "required but default",
			manifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Config: manifestsv1alpha1.PackageManifestSpecConfig{
						OpenAPIV3Schema: nil,
					},
				},
			},
			configuration: map[string]interface{}{
				"k1": "v1", "k2": "v2",
			},
			expectedConfiguration: map[string]interface{}{},
			expectedErrors:        []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pac := NewPackageAdmissionController(testScheme)
			pac.packageManifestValidator = func(ctx context.Context, s *runtime.Scheme, pm *manifestsv1alpha1.PackageManifest) field.ErrorList {
				return nil
			}

			ctx := context.Background()
			err := pac.Admit(ctx, test.configuration, test.manifest)

			var (
				errorStrings []string
				aerr         utilerrors.Aggregate
			)
			if errors.As(err, &aerr) {
				for _, err := range aerr.Errors() {
					errorStrings = append(errorStrings, err.Error())
				}
			} else if err != nil {
				errorStrings = append(errorStrings, err.Error())
			}

			assert.Len(t, errorStrings, len(test.expectedErrors))
			for _, expectedError := range test.expectedErrors {
				assert.Contains(t, errorStrings, expectedError)
			}

			if len(test.expectedErrors) == 0 {
				assert.Equal(t, test.expectedConfiguration, test.configuration)
			}
		})
	}
}
