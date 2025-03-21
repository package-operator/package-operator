package kickstart

import (
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

	ctx := t.Context()
	k := NewKickstarter(nil)
	msg, err := k.Kickstart(ctx, "my-pkg", []string{"testdata/all-the-objects.yaml"}, "", nil)
	require.NoError(t, err)
	assert.Equal(t, kickstartMessage, msg)
}
