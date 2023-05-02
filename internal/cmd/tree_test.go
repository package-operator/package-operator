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
		"valid source/no options": {
			SourcePath: "testdata",
			Assertion:  require.NoError,
			ExpectedOutput: strings.Join([]string{
				"test-stub",
				"Package test-ns/test",
				"",
			}, "\n"),
		},
		"valid source/cluster scope": {
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
		"invalid source": {
			SourcePath: "dne",
			Assertion:  require.Error,
		},
	} {
		tc := tc

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
