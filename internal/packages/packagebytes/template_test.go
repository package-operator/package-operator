package packagebytes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TestTemplateTransformer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tt := &TemplateTransformer{
			TemplateContext: manifestsv1alpha1.TemplateContext{
				Package: manifestsv1alpha1.TemplateContextPackage{
					TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{
						Name: "test",
					},
				},
			},
		}

		template := []byte("#{{.Package.Name}}#")
		fm := FileMap{
			"something":        template,
			"something.yaml":   template,
			"test.yaml.gotmpl": template,
			"test.yml.gotmpl":  template,
		}

		ctx := context.Background()
		err := tt.Transform(ctx, fm)
		require.NoError(t, err)

		templateResult := "#test#"
		assert.Equal(t, templateResult, string(fm["test.yaml"]))
		assert.Equal(t, templateResult, string(fm["test.yml"]))
		// only touches YAML files
		assert.Equal(t, string(template), string(fm["something"]))
	})

	t.Run("invalid template", func(t *testing.T) {
		tt := &TemplateTransformer{
			TemplateContext: manifestsv1alpha1.TemplateContext{
				Package: manifestsv1alpha1.TemplateContextPackage{
					TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{
						Name: "test",
					},
				},
			},
		}

		template := []byte("#{{.Package.Name}#")
		fm := FileMap{
			"test.yaml.gotmpl": template,
		}

		ctx := context.Background()
		err := tt.Transform(ctx, fm)
		require.Error(t, err)
	})

	t.Run("execution template error", func(t *testing.T) {
		tt := &TemplateTransformer{
			TemplateContext: manifestsv1alpha1.TemplateContext{
				Package: manifestsv1alpha1.TemplateContextPackage{
					TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{
						Name: "test",
					},
				},
			},
		}

		template := []byte("#{{.Package.Banana}}#")
		fm := FileMap{
			"test.yaml.gotmpl": template,
		}

		ctx := context.Background()
		err := tt.Transform(ctx, fm)
		require.Error(t, err)
	})
}

func TestTemplateTestValidator(t *testing.T) {
	fixturesPath := filepath.Join("testdata", "ttv")
	defer func() {
		err := os.RemoveAll(fixturesPath)
		require.NoError(t, err) // start clean
	}()

	pcl := &PackageContentLoaderMock{}
	pml := &PackageManifestLoaderMock{}

	packageManifest := &manifestsv1alpha1.PackageManifest{
		Test: manifestsv1alpha1.PackageManifestTest{
			Template: []manifestsv1alpha1.PackageManifestTestCaseTemplate{
				{
					Name: "t1",
					Context: manifestsv1alpha1.TemplateContext{
						Package: manifestsv1alpha1.TemplateContextPackage{
							TemplateContextObjectMeta: manifestsv1alpha1.
								TemplateContextObjectMeta{
								Name:      "pkg-name",
								Namespace: "pkg-namespace",
							},
						},
					},
				},
			},
		},
	}

	pml.
		On("FromFileMap", mock.Anything, mock.Anything).
		Return(packageManifest, nil)

	pcl.
		On("Load", mock.Anything, mock.Anything).
		Return(nil)

	ctx := logr.NewContext(context.Background(), testr.New(t))
	ttv := NewTemplateTestValidator(fixturesPath, pcl.Load, pml)

	originalFileMap := FileMap{
		"file2.yaml.gotmpl": []byte("{{.Package.Namespace}}\n"),
		"file.yaml.gotmpl":  []byte("{{.Package.Name}}\n"),
	}
	err := ttv.Validate(ctx, originalFileMap)
	require.NoError(t, err)

	// Assert Fixtures have been setup

	newFileMap := FileMap{
		"file2.yaml.gotmpl": []byte("{{.Package.Namespace}}\n"),
		"file.yaml.gotmpl":  []byte("#{{.Package.Name}}#\n"),
	}
	expectedErr := `Package validation errors:
- Test "t1": File mismatch against fixture in file.yaml.gotmpl:
  --- FIXTURE/file.yaml
  +++ ACTUAL/file.yaml
  @@ -1 +1 @@
  -pkg-name
  +#pkg-name#`
	err = ttv.Validate(ctx, newFileMap)
	require.Equal(t, expectedErr, err.Error())
}

type PackageContentLoaderMock struct {
	mock.Mock
}

func (m *PackageContentLoaderMock) Load(ctx context.Context, fileMap FileMap) error {
	args := m.Called(ctx, fileMap)
	return args.Error(0)
}

type PackageManifestLoaderMock struct {
	mock.Mock
}

func (m *PackageManifestLoaderMock) FromFileMap(ctx context.Context, fileMap FileMap) (
	*manifestsv1alpha1.PackageManifest, error,
) {
	args := m.Called(ctx, fileMap)
	return args.Get(0).(*manifestsv1alpha1.PackageManifest),
		args.Error(1)
}
