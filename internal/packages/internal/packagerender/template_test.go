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
			Files: fm,
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
			Files: fm,
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
			Files: fm,
		}

		ctx := context.Background()
		err := RenderTemplates(ctx, pkg, tmplCtx)
		require.Error(t, err)
	})
}
