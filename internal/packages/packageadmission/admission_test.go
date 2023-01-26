package packageadmission_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/packageadmission"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
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

func TestValidatePackageManifest(t *testing.T) {
	tests := []struct {
		name            string
		packageManifest *manifestsv1alpha1.PackageManifest
		expectedErrors  []string
	}{
		{
			name:            "empty",
			packageManifest: &manifestsv1alpha1.PackageManifest{},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.availabilityProbes: Required value",
			},
		},
		{
			name: "duplicated phase",
			packageManifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Phases: []manifestsv1alpha1.PackageManifestPhase{
						{Name: "test"},
						{Name: "test"},
					},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases[1].name: Invalid value: \"test\": must be unique",
				"spec.availabilityProbes: Required value",
			},
		},
		{
			name: "openAPI invalid template context",
			packageManifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Config: manifestsv1alpha1.PackageManifestSpecConfig{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:     packageadmission.OpenapiV3TypeObject,
							Required: []string{"banana"},
						},
					},
				},
				Test: manifestsv1alpha1.PackageManifestTest{
					Template: []manifestsv1alpha1.PackageManifestTestCaseTemplate{
						{
							Name:    "Invalid",
							Context: manifestsv1alpha1.TemplateContext{},
						},
					},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.availabilityProbes: Required value",
				"test.template[0].context.config.banana: Required value",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ferrs := packageadmission.ValidatePackageManifest(ctx, testScheme, test.packageManifest)
			require.Len(t, ferrs, len(test.expectedErrors))

			var errorStrings []string
			for _, err := range ferrs {
				errorStrings = append(errorStrings, err.Error())
			}
			for _, expectedError := range test.expectedErrors {
				assert.Contains(t, errorStrings, expectedError)
			}
		})
	}
}

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
					Type: packageadmission.OpenapiV3TypeObject,
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
			ferrs, err := packageadmission.ValidatePackageConfiguration(ctx, testScheme, test.packageManifestConfig, config, nil)
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

func TestPackageManifest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		manifest    *manifestsv1alpha1.PackageManifest
		errorString string // main error string that we are interested in, might return more.
	}{
		{
			name:        "missing .metadata.name",
			manifest:    &manifestsv1alpha1.PackageManifest{},
			errorString: "metadata.name: Required value",
		},
		{
			name:        "missing .spec.scopes",
			manifest:    &manifestsv1alpha1.PackageManifest{},
			errorString: "spec.scopes: Required value",
		},
		{
			name:        "missing .spec.availabilityProbes",
			manifest:    &manifestsv1alpha1.PackageManifest{},
			errorString: "spec.availabilityProbes: Required value",
		},
		{
			name:        "missing .spec.phases",
			manifest:    &manifestsv1alpha1.PackageManifest{},
			errorString: "spec.phases: Required value",
		},
		{
			name: "duplicated phase",
			manifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Phases: []manifestsv1alpha1.PackageManifestPhase{
						{Name: "test"},
						{Name: "test"},
					},
				},
			},
			errorString: "spec.phases[1].name: Invalid value: \"test\": must be unique",
		},
		{
			name: "duplicated phase",
			manifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
						{},
					},
				},
			},
			errorString: "spec.availabilityProbes[0].probes: Required value",
		},
	}

	ctx := context.Background()

	for _, test := range tests {
		thisTest := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := packageadmission.ValidatePackageManifest(ctx, testScheme, thisTest.manifest)

			var errorStrings []string
			for _, err := range err {
				errorStrings = append(errorStrings, err.Error())
			}

			assert.Contains(t, errorStrings, thisTest.errorString)
		})
	}
}
