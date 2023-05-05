package deps

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	t.Parallel()

	deps, err := Build()
	require.NoError(t, err)

	require.NoError(t, deps.Invoke(func(rootCmd *cobra.Command) {
		assert.NotEmpty(t, rootCmd.Commands())
	}))
}
