package packagemanifestvalidation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"package-operator.run/internal/apis/manifests"
)

func TestValidatePackageManifest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		packageManifest *manifests.PackageManifest
		expectedErrors  []string
	}{
		{
			name:            "empty",
			packageManifest: &manifests.PackageManifest{},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
			},
		},
		{
			name: "invalid constraints",
			packageManifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Constraints: []manifests.PackageManifestConstraint{
						{
							PlatformVersion: &manifests.PackageManifestPlatformVersionConstraint{
								Name:  "OpenShift",
								Range: "banana",
							},
						},
					},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				`spec.constraints[0].platformVersion.range: Invalid value: "banana": improper constraint`,
			},
		},
		{
			name: "duplicated phase",
			packageManifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Phases: []manifests.PackageManifestPhase{
						{Name: "test"},
						{Name: "test"},
					},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases[1].name: Invalid value: \"test\": must be unique",
			},
		},
		{
			name: "openAPI invalid template context",
			packageManifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Config: manifests.PackageManifestSpecConfig{
						OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
							Type:     OpenapiV3TypeObject,
							Required: []string{"banana"},
						},
					},
				},
				Test: manifests.PackageManifestTest{
					Template: []manifests.PackageManifestTestCaseTemplate{
						{
							Name:    "Invalid",
							Context: manifests.TemplateContext{},
						},
					},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"test.template[0].context.config.banana: Required value",
			},
		},
		{
			name: "empty image",
			packageManifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Images: []manifests.PackageManifestImage{{}},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.images[0].name: Invalid value: \"\": must be non empty",
				"spec.images[0].image: Invalid value: \"\": must be non empty",
			},
		},
		{
			name: "empty image name",
			packageManifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Images: []manifests.PackageManifestImage{{Image: "nginx:1.23.3"}},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.images[0].name: Invalid value: \"\": must be non empty",
			},
		},
		{
			name: "empty image identifier",
			packageManifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Images: []manifests.PackageManifestImage{{Name: "nginx"}},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.images[0].image: Invalid value: \"\": must be non empty",
			},
		},
		{
			name: "duplicated image name",
			packageManifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Images: []manifests.PackageManifestImage{
						{Name: "nginx", Image: "nginx:1.23.3"},
						{Name: "nginx", Image: "nginx:1.22.1"},
					},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"spec.images[1].name: Invalid value: \"nginx\": must be unique",
			},
		},
		{
			name: "kubeconform missing kubernetesVersion",
			packageManifest: &manifests.PackageManifest{
				Test: manifests.PackageManifestTest{
					Kubeconform: &manifests.PackageManifestTestKubeconform{},
				},
			},
			expectedErrors: []string{
				"metadata.name: Required value",
				"spec.scopes: Required value",
				"spec.phases: Required value",
				"test.kubeconform.kubernetesVersion: Required value",
			},
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ferrs, err := ValidatePackageManifest(ctx, test.packageManifest)
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
