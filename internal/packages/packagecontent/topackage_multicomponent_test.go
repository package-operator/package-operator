package packagecontent_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageimport"
)

type testFile struct {
	name    string
	content []byte
}

type testData struct {
	directory string
	component string
	file      *testFile
	error     error
}

func TestMultiComponentLoader(t *testing.T) {
	t.Parallel()

	tests := []testData{
		{"components-disabled", "", nil, nil},
		{"components-disabled", "foobar", nil, packages.ViolationError{Reason: packages.ViolationReasonComponentsNotEnabled}},

		{"components-enabled/not-dns1123", "_backend", nil, packages.ViolationError{
			Reason: packages.ViolationReasonInvalidComponentPath,
			Path:   "components/_backend/Deployment.yaml",
		}},
		{"components-enabled/nested-components", "", nil, packages.ViolationError{
			Reason:    packages.ViolationReasonNestedMultiComponentPkg,
			Path:      "manifest.yaml",
			Component: "backend",
		}},

		{"components-enabled/valid", "", nil, nil},
		{"components-enabled/valid", "backend", nil, nil},
		{"components-enabled/valid", "frontend", nil, nil},
		{"components-enabled/valid", "foobar", nil, packages.ViolationError{Reason: packages.ViolationReasonComponentNotFound, Component: "foobar"}},
		{"components-enabled/valid", "frontend", nil, nil},

		{"components-enabled/valid", "", &testFile{
			"components/banana.txt",
			[]byte("bread"),
		}, packages.ViolationError{
			Reason: packages.ViolationReasonInvalidFileInComponentsDir,
			Path:   "components/banana.txt",
		}},
		{"components-enabled/valid", "", &testFile{
			"components/.sneaky-banana.txt",
			[]byte("bread"),
		}, nil},
		{"components-enabled/valid", "", &testFile{
			"components/backend/manifest.yml",
			[]byte("apiVersion: manifests.package-operator.run/v1alpha1\nkind: PackageManifest\nmetadata:\n  name: application\nspec:\n  scopes:\n    - Namespaced\n  phases:\n    - name: configure"),
		}, packages.ViolationError{
			Reason: packages.ViolationReasonPackageManifestDuplicated,
			Path:   "manifest.yml",
		}},
	}

	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			files, err := packageimport.Folder(ctx, filepath.Join("testdata", "multi-component", test.directory))
			require.NoError(t, err)

			if test.file != nil {
				files[test.file.name] = test.file.content
			}

			pkg, err := packagecontent.AllPackagesFromFiles(ctx, testScheme, files, test.component)

			if test.error == nil {
				require.NoError(t, err)
				require.NotNil(t, pkg)
			} else {
				require.EqualError(t, err, test.error.Error())
			}
		})
	}
}
