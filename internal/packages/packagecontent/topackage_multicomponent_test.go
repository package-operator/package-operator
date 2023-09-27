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
		{"components-enabled-invalid", "_backend", packages.ViolationError{
			Reason: packages.ViolationReasonInvalidComponentPath,
			Path:   "components/_backend/Deployment.yaml",
		}},
		{"components-enabled-valid", "", nil},
		{"components-enabled-valid", "backend", nil},
		{"components-enabled-valid", "frontend", nil},
		{"components-enabled-valid", "foobar", packages.ViolationError{Reason: packages.ViolationReasonComponentNotFound, Component: "foobar"}},
	}

	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			files, err := packageimport.Folder(ctx, filepath.Join("testdata", "multi-component", test.directory))
			require.NoError(t, err)

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
