package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTree_RenderPackage(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		SourcePath     string
		Options        []RenderPackageOption
		Assertion      require.ErrorAssertionFunc
		ExpectedOutput string
	}{
		"simple/valid source/no options": {
			SourcePath: "testdata",
			Assertion:  require.NoError,
			ExpectedOutput: strings.Join([]string{
				"test-stub",
				"Package test-ns/test",
				"",
			}, "\n"),
		},
		"simple/valid source/cluster scope": {
			SourcePath: "testdata",
			Options: []RenderPackageOption{
				WithClusterScope(true),
			},
			Assertion: require.NoError,
			ExpectedOutput: strings.Join([]string{
				"test-stub",
				"ClusterPackage /test",
				"",
			}, "\n"),
		},
		"simple/invalid source": {
			SourcePath: "dne",
			Assertion:  require.Error,
		},
		"multi/valid package source/no options": {
			SourcePath: "../testutil/testdata/multi-with-config",
			Assertion:  require.NoError,
			ExpectedOutput: strings.Join([]string{
				"test-webapp",
				"Package namespace/name",
				"└── Phase deploy-backend",
				"│   ├── package-operator.run/v1alpha1, Kind=Package /name-backend",
				"└── Phase deploy-frontend",
				"    └── package-operator.run/v1alpha1, Kind=Package /name-frontend",
				"",
			}, "\n"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme, err := NewScheme()
			require.NoError(t, err)

			tree := NewTree(scheme)

			output, err := tree.RenderPackage(context.Background(), tc.SourcePath, tc.Options...)
			tc.Assertion(t, err)

			assert.Equal(t, tc.ExpectedOutput, output)
		})
	}
}
