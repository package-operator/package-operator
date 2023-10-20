package packagemanifestvalidation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/apis/manifests"
)

func TestValidatePackageConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		packageManifestConfig *manifests.PackageManifestSpecConfig
		config                map[string]interface{}
		expectedErrors        []string
	}{
		{
			name: "wrong type and missing required",
			packageManifestConfig: &manifests.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
					Type: OpenapiV3TypeObject,
					Properties: map[string]apiextensions.JSONSchemaProps{
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
			ferrs, err := ValidatePackageConfiguration(ctx, test.packageManifestConfig, test.config, nil)
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
		manifest    *manifests.PackageManifest
		errorString string // main error string that we are interested in, might return more.
	}{
		{
			name:        "missing .metadata.name",
			manifest:    &manifests.PackageManifest{},
			errorString: "metadata.name: Required value",
		},
		{
			name:        "missing .spec.scopes",
			manifest:    &manifests.PackageManifest{},
			errorString: "spec.scopes: Required value",
		},
		{
			name:        "missing .spec.phases",
			manifest:    &manifests.PackageManifest{},
			errorString: "spec.phases: Required value",
		},
		{
			name: "duplicated phase",
			manifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Phases: []manifests.PackageManifestPhase{
						{Name: "test"},
						{Name: "test"},
					},
				},
			},
			errorString: "spec.phases[1].name: Invalid value: \"test\": must be unique",
		},
		{
			name: "missing probes in availabilityProbes",
			manifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
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

			ferr, err := ValidatePackageManifest(ctx, test.manifest)
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
	chicken := apiextensions.JSON(`🐔`)
	man := &manifests.PackageManifest{
		Spec: manifests.PackageManifestSpec{
			Config: manifests.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"chicken": {Default: &chicken},
					},
				},
			},
		},
	}
	elist, err := AdmitPackageConfiguration(ctx, inputCfg, man, field.NewPath("spec", "config"))
	require.NoError(t, err)
	require.Nil(t, elist)
	require.Equal(t, expectedOutputConfig, inputCfg)
}

func TestAdmitPackageConfiguration_Default(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	inputCfg := map[string]interface{}{"banana": "🍌"}
	expectedOutputConfig := map[string]interface{}{"chicken": "🐔"}
	chicken := apiextensions.JSON(`🐔`)
	man := &manifests.PackageManifest{
		Spec: manifests.PackageManifestSpec{
			Config: manifests.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"chicken": {Default: &chicken},
					},
				},
			},
		},
	}
	elist, err := AdmitPackageConfiguration(ctx, inputCfg, man, field.NewPath("spec", "config"))
	require.NoError(t, err)
	require.Nil(t, elist)
	require.Equal(t, expectedOutputConfig, inputCfg)
}

func TestAdmitPackageConfigurationTemplating_Default(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	inputCfg := map[string]interface{}{"banana": "🍌"}
	expectedOutputConfig := map[string]interface{}{"chicken": "🐔"}
	chicken := apiextensions.JSON(`🐔`)
	man := &manifests.PackageManifest{
		Spec: manifests.PackageManifestSpec{
			Config: manifests.PackageManifestSpecConfig{
				OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
					Required: []string{"chicken"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"chicken": {Default: &chicken},
					},
				},
			},
		},
		Test: manifests.PackageManifestTest{
			Template: []manifests.PackageManifestTestCaseTemplate{{
				Name: "verytestytestcase",
				Context: manifests.TemplateContext{
					Package: manifests.TemplateContextPackage{},
					Config:  &runtime.RawExtension{},
				},
			}},
		},
	}
	elist, err := AdmitPackageConfiguration(ctx, inputCfg, man, field.NewPath("spec", "config"))
	require.NoError(t, err)
	require.Nil(t, elist)
	require.Equal(t, expectedOutputConfig, inputCfg)
}
