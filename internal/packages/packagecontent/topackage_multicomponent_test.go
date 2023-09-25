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
		{"components-enabled", "", nil},
		{"components-enabled", "backend", nil},
		{"components-enabled", "frontend", nil},
		{"components-enabled", "foobar", packages.ErrManifestNotFound},
	}

	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			files, err := packageimport.Folder(ctx, filepath.Join("testdata", "multi-component", test.directory))
			require.NoError(t, err)

			pkg, err := packagecontent.PackageFromFiles(ctx, testScheme, files, test.component)

			if test.error == nil {
				require.NoError(t, err)
				require.NotNil(t, pkg)
			} else {
				require.ErrorIs(t, err, test.error)
			}
		})
	}
}
