package kickstart

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKickstart(t *testing.T) {
	defer os.RemoveAll("my-pkg")

	ctx := context.Background()
	k := NewKickstarter(nil)
	msg, err := k.KickStart(ctx, "my-pkg", []string{"testdata/all-the-objects.yaml"})
	require.NoError(t, err)
	assert.Equal(t, `Kickstarted the "my-pkg" package with 2 objects.`, msg)
}
