package packageadmission_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/packageadmission"
)

func TestValidatePackageConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		packageManifestConfig *manifestsv1alpha1.PackageManifestSpecConfig
		config                map[string]interface{}
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
			config: map[string]interface{}{"test": float64(42)},
			expectedErrors: []string{
				`test: Invalid value: "number": test in body must be of type string: "number"`,
				"banana: Required value",
			},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ferrs, err := packageadmission.ValidatePackageConfiguration(ctx, testScheme, test.packageManifestConfig, test.config, nil)
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
			name: "missing probes in availabilityProbes",
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

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ferr, err := packageadmission.ValidatePackageManifest(ctx, testScheme, test.manifest)
			require.NoError(t, err)

			var errorStrings []string
			for _, err := range ferr {
				errorStrings = append(errorStrings, err.Error())
			}

			assert.Contains(t, errorStrings, test.errorString)
		})
	}
}

func TestAdmitPackageConfiguration_Prune(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	inputCfg := map[string]interface{}{"chicken": "🐔", "banana": "🍌"}
	expectedOutputConfig := map[string]interface{}{"chicken": "🐔"}
	man := &manifestsv1alpha1.PackageManifest{
		Spec: manifestsv1alpha1.PackageManifestSpec{
			Config: manifestsv1alpha1.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"chicken": {Default: &apiextensionsv1.JSON{Raw: []byte(`"🐔"`)}},
					},
				},
			},
		},
	}
	elist, err := packageadmission.AdmitPackageConfiguration(ctx, testScheme, inputCfg, man, field.NewPath("spec", "config"))
	require.NoError(t, err)
	require.Nil(t, elist)
	require.Equal(t, expectedOutputConfig, inputCfg)
}

func TestAdmitPackageConfiguration_Default(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	inputCfg := map[string]interface{}{"banana": "🍌"}
	expectedOutputConfig := map[string]interface{}{"chicken": "🐔"}
	man := &manifestsv1alpha1.PackageManifest{
		Spec: manifestsv1alpha1.PackageManifestSpec{
			Config: manifestsv1alpha1.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"chicken": {Default: &apiextensionsv1.JSON{Raw: []byte(`"🐔"`)}},
					},
				},
			},
		},
	}
	elist, err := packageadmission.AdmitPackageConfiguration(ctx, testScheme, inputCfg, man, field.NewPath("spec", "config"))
	require.NoError(t, err)
	require.Nil(t, elist)
	require.Equal(t, expectedOutputConfig, inputCfg)
}

func TestAdmitPackageConfigurationTemplating_Default(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	inputCfg := map[string]interface{}{"banana": "🍌"}
	expectedOutputConfig := map[string]interface{}{"chicken": "🐔"}
	man := &manifestsv1alpha1.PackageManifest{
		Spec: manifestsv1alpha1.PackageManifestSpec{
			Config: manifestsv1alpha1.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Required: []string{"chicken"},
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"chicken": {Default: &apiextensionsv1.JSON{Raw: []byte(`"🐔"`)}},
					},
				},
			},
		},
		Test: manifestsv1alpha1.PackageManifestTest{
			Template: []manifestsv1alpha1.PackageManifestTestCaseTemplate{{
				Name: "verytestytestcase",
				Context: manifestsv1alpha1.TemplateContext{
					Package: manifestsv1alpha1.TemplateContextPackage{},
					Config:  &runtime.RawExtension{},
				},
			}},
		},
	}
	elist, err := packageadmission.AdmitPackageConfiguration(ctx, testScheme, inputCfg, man, field.NewPath("spec", "config"))
	require.NoError(t, err)
	require.Nil(t, elist)
	require.Equal(t, expectedOutputConfig, inputCfg)
}
