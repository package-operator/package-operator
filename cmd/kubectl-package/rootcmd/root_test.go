package rootcmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvideRootCmd(t *testing.T) {
	t.Parallel()

	params := Params{
		Streams: IOStreams{
			In:     &bytes.Buffer{},
			Out:    &bytes.Buffer{},
			ErrOut: &bytes.Buffer{},
		},
		Args: []string{},
	}

	cmd := ProvideRootCmd(params)

	assert.Same(t, params.Streams.In, cmd.InOrStdin())
	assert.Same(t, params.Streams.Out, cmd.OutOrStdout())
	assert.Same(t, params.Streams.ErrOut, cmd.ErrOrStderr())
}
