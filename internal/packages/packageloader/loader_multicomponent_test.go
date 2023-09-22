package packageloader_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"package-operator.run/internal/packages"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/packageimport"
	"package-operator.run/internal/packages/packageloader"
)

type testData struct {
	directory string
	component string
	error     error
}

func TestMultiComponentLoader(t *testing.T) {
	t.Parallel()

	tests := []testData{
		{"components-disabled", "", nil},
		{"components-disabled", "foobar", packages.ViolationError{Reason: packages.ViolationReasonComponentsNotEnabled}},
		{"components-enabled", "", nil},
		{"components-enabled", "backend", nil},
		{"components-enabled", "frontend", nil},
		{"components-enabled", "foobar", packages.ErrManifestNotFound},
	}

	for i, tst := range tests {
		test := tst
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()

			transformer, err := packageloader.NewTemplateTransformer(
				packageloader.PackageFileTemplateContext{
					Package: manifestsv1alpha1.TemplateContextPackage{
						TemplateContextObjectMeta: manifestsv1alpha1.TemplateContextObjectMeta{Namespace: "test123-ns"},
					},
				},
			)
			require.NoError(t, err)

			var l *packageloader.Loader
			if test.component == "" {
				l = packageloader.New(testScheme, packageloader.WithDefaults, packageloader.WithFilesTransformers(transformer))
			} else {
				l = packageloader.New(testScheme, packageloader.WithDefaults, packageloader.WithFilesTransformers(transformer), packageloader.WithComponent(test.component))
			}

			ctx := logr.NewContext(context.Background(), testr.New(t))
			files, err := packageimport.Folder(ctx, filepath.Join("testdata", "multi-component", test.directory))
			require.NoError(t, err)

			_, err = l.FromFiles(ctx, files)

			if test.error == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.error)
			}
		})
	}
}
