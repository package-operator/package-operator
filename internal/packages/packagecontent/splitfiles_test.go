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

func TestMultiComponentLoader(t *testing.T) {
	t.Parallel()

	for i, test := range []struct {
		directories []string
		component   string
		file        *testFile
		errors      []error
	}{
		{[]string{"components-disabled"}, "", nil, nil},
		{[]string{"components-disabled"}, "foobar", nil, []error{packages.ViolationError{Reason: packages.ViolationReasonComponentsNotEnabled}}},

		{[]string{"components-enabled/not-dns1123"}, "_backend", nil, []error{packages.ViolationError{
			Reason: packages.ViolationReasonInvalidComponentPath,
			Path:   "components/_backend/Deployment.yaml",
		}}},
		{[]string{"components-enabled/nested-components"}, "", nil, []error{packages.ViolationError{
			Reason:    packages.ViolationReasonNestedMultiComponentPkg,
			Component: "backend",
		}}},

		{[]string{"components-enabled/valid", "multi-with-config"}, "", nil, nil},
		{[]string{"components-enabled/valid", "multi-with-config"}, "backend", nil, nil},
		{[]string{"components-enabled/valid", "multi-with-config"}, "frontend", nil, nil},
		{[]string{"components-enabled/valid", "multi-with-config"}, "foobar", nil, []error{packages.ViolationError{Reason: packages.ViolationReasonComponentNotFound, Component: "foobar"}}},

		{[]string{"components-enabled/valid", "multi-with-config"}, "", &testFile{
			"components/banana.txt",
			[]byte("bread"),
		}, []error{packages.ViolationError{
			Reason: packages.ViolationReasonInvalidFileInComponentsDir,
			Path:   "components/banana.txt",
		}}},
		{[]string{"components-enabled/valid", "multi-with-config"}, "", &testFile{
			"components/.sneaky-banana.txt",
			[]byte("bread"),
		}, nil},
		{[]string{"components-enabled/valid", "multi-with-config"}, "", &testFile{
			"components/backend/manifest.yml",
			[]byte("apiVersion: manifests.package-operator.run/v1alpha1\nkind: PackageManifest\nmetadata:\n  name: application\nspec:\n  scopes:\n    - Namespaced\n  phases:\n    - name: configure"),
		}, []error{
			packages.ViolationError{
				Reason: packages.ViolationReasonPackageManifestDuplicated,
				Path:   "manifest.yaml",
			},
			packages.ViolationError{
				Reason: packages.ViolationReasonPackageManifestDuplicated,
				Path:   "manifest.yml",
			},
		}},
	} {
		test := test
		for _, directory := range test.directories {
			directory := directory
			t.Run(fmt.Sprintf("%02d/%s/%s", i, directory, test.component), func(t *testing.T) {
				t.Parallel()

				ctx := context.Background()
				files, err := packageimport.Folder(ctx, filepath.Join("testdata", "multi-component", directory))
				if err != nil {
					files, err = packageimport.Folder(ctx, filepath.Join("..", "..", "testutil", "testdata", directory))
				}
				require.NoError(t, err)

				if test.file != nil {
					files[test.file.name] = test.file.content
				}

				pkg, err := packagecontent.SplitFilesByComponent(ctx, testScheme, files, test.component)

				if test.errors == nil || len(test.errors) == 0 {
					require.NoError(t, err)
					require.NotNil(t, pkg)
				} else {
					requireErrEqualsOneOf(t, err, test.errors)
				}
			})
		}
	}
}

func requireErrEqualsOneOf(t *testing.T, err error, targets []error) {
	t.Helper()

	isOneOf := false
	for _, target := range targets {
		if err.Error() == target.Error() {
			isOneOf = true
			break
		}
	}
	require.True(t, isOneOf, "require error message to match one of target errors",
		"err", err,
		"targets", targets,
	)
}
