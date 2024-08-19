package packagerender

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestRenderTemplates(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tmplCtx := packagetypes.PackageRenderContext{
			Package: manifests.TemplateContextPackage{
				TemplateContextObjectMeta: manifests.TemplateContextObjectMeta{
					Name: "test",
				},
			},
		}

		template := []byte("#{{.package.metadata.name}}#")
		fm := packagetypes.Files{
			"something":        template,
			"something.yaml":   template,
			"test.yaml.gotmpl": template,
			"test.yml.gotmpl":  template,
		}
		pkg := &packagetypes.Package{
			Files:    fm,
			Manifest: &manifests.PackageManifest{},
		}

		ctx := context.Background()
		err := RenderTemplates(ctx, pkg, tmplCtx)
		require.NoError(t, err)

		templateResult := "#test#"
		assert.Equal(t, templateResult, string(fm["test.yaml"]))
		assert.Equal(t, templateResult, string(fm["test.yml"]))
		// only touches YAML files
		assert.Equal(t, string(template), string(fm["something"]))
	})

	t.Run("invalid template", func(t *testing.T) {
		t.Parallel()
		tmplCtx := packagetypes.PackageRenderContext{
			Package: manifests.TemplateContextPackage{
				TemplateContextObjectMeta: manifests.TemplateContextObjectMeta{
					Name: "test",
				},
			},
		}

		template := []byte("#{{.package.metadata.name}#")
		fm := packagetypes.Files{
			"test.yaml.gotmpl": template,
		}
		pkg := &packagetypes.Package{
			Files:    fm,
			Manifest: &manifests.PackageManifest{},
		}

		ctx := context.Background()
		err := RenderTemplates(ctx, pkg, tmplCtx)
		require.Error(t, err)
	})

	t.Run("execution template error", func(t *testing.T) {
		t.Parallel()

		tmplCtx := packagetypes.PackageRenderContext{
			Package: manifests.TemplateContextPackage{
				TemplateContextObjectMeta: manifests.TemplateContextObjectMeta{
					Name: "test",
				},
			},
		}

		template := []byte("#{{.Package.Banana}}#")
		fm := packagetypes.Files{
			"test.yaml.gotmpl": template,
		}
		pkg := &packagetypes.Package{
			Files:    fm,
			Manifest: &manifests.PackageManifest{},
		}

		ctx := context.Background()
		err := RenderTemplates(ctx, pkg, tmplCtx)
		require.Error(t, err)
	})
}

func TestRenderTemplates_CelFunction(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		tmplCtx    packagetypes.PackageRenderContext
		conditions []manifests.PackageManifestNamedCondition
		template   string
		result     string
		err        error
	}{
		{
			name:     "if else with cel",
			template: `{{if cel "true && false"}}then{{else}}else{{end}}`,
			result:   "else",
		},
		{
			name: "context construction error",
			conditions: []manifests.PackageManifestNamedCondition{
				{
					Name:       "invalid",
					Expression: "invalid",
				},
			},
			template: `{{cel "cond.invalid"}}`,
			err:      errConstructingCelContext,
		},
		{
			name: "reusable condition",
			conditions: []manifests.PackageManifestNamedCondition{
				{
					Name:       "test_condition",
					Expression: "true && false",
				},
			},
			template: `{{cel "cond.test_condition"}}`,
			result:   "false",
		},
		{
			name: "template context access",
			tmplCtx: packagetypes.PackageRenderContext{
				Config: map[string]any{
					"banana": "bread",
				},
			},
			template: `{{cel ".config.banana == \"bread\""}}`,
			result:   "true",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fm := packagetypes.Files{
				"test.yaml.gotmpl": []byte(tc.template),
			}
			pkg := &packagetypes.Package{
				Files: fm,
				Manifest: &manifests.PackageManifest{
					Spec: manifests.PackageManifestSpec{
						Filters: manifests.PackageManifestFilter{
							Conditions: tc.conditions,
						},
					},
				},
			}

			ctx := context.Background()
			err := RenderTemplates(ctx, pkg, tc.tmplCtx)

			if tc.err == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.result, string(fm["test.yaml"]))
			} else {
				assert.ErrorIs(t, err, tc.err)
			}
		})
	}
}
