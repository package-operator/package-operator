package kickstart

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var kickstartMessage = `Kickstarted the "my-pkg" package with 3 objects.
[WARN] Some kinds don't have availability probes defined:
- Banana.fruits
`

func TestKickstart(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := os.RemoveAll("my-pkg"); err != nil {
			panic(err)
		}
	}()

	ctx := context.Background()
	k := NewKickstarter(nil)
	msg, err := k.KickStart(ctx, "my-pkg", []string{"testdata/all-the-objects.yaml"})
	require.NoError(t, err)
	assert.Equal(t, kickstartMessage, msg)
}
