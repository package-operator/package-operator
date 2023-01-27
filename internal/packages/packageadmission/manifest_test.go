package packageadmission_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/packageadmission"
)

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
		{
			name: "empty image",
			packageManifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Images: []manifestsv1alpha1.PackageManifestImage{{}},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.availabilityProbes: Required value",
				"spec.images[0].name: Invalid value: \"\": must be non empty",
				"spec.images[0].image: Invalid value: \"\": must be non empty",
			},
		},
		{
			name: "empty image name",
			packageManifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Images: []manifestsv1alpha1.PackageManifestImage{{Image: "nginx:latest"}},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.availabilityProbes: Required value",
				"spec.images[0].name: Invalid value: \"\": must be non empty",
			},
		},
		{
			name: "empty image identifier",
			packageManifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Images: []manifestsv1alpha1.PackageManifestImage{{Name: "nginx"}},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.availabilityProbes: Required value",
				"spec.images[0].image: Invalid value: \"\": must be non empty",
			},
		},
		{
			name: "duplicated image name",
			packageManifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Images: []manifestsv1alpha1.PackageManifestImage{
						{Name: "nginx", Image: "nginx:latest"},
						{Name: "nginx", Image: "nginx:stable"},
					},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.availabilityProbes: Required value",
				"spec.images[1].name: Invalid value: \"nginx\": must be unique",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ferrs, err := packageadmission.ValidatePackageManifest(ctx, testScheme, test.packageManifest)
			require.NoError(t, err)
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
