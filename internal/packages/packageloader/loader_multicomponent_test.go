package packageloader_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/packageimport"
	"package-operator.run/internal/packages/packageloader"
)

func TestMultiComponentLoader(t *testing.T) {
	t.Parallel()

	transformer, err := packageloader.NewTemplateTransformer(
		packageloader.PackageFileTemplateContext{
			Package: manifestsv1alpha1.TemplateContextPackage{
				TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{Namespace: "test123-ns"},
			},
		},
	)
	require.NoError(t, err)

	l := packageloader.New(testScheme, packageloader.WithDefaults, packageloader.WithFilesTransformers(transformer))

	ctx := logr.NewContext(context.Background(), testr.New(t))
	files, err := packageimport.Folder(ctx, filepath.Join("testdata", "multi-component", "000-ok-components-disabled"))
	require.NoError(t, err)

	_, err = l.FromFiles(ctx, files)
	require.NoError(t, err)
}
