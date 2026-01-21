package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithOut_ConfigurePrinter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	withOut := WithOut{Out: &buf}
	config := &PrinterConfig{}

	withOut.ConfigurePrinter(config)

	assert.Equal(t, &buf, config.Out)
}

func TestWithErr_ConfigurePrinter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	withErr := WithErr{Err: &buf}
	config := &PrinterConfig{}

	withErr.ConfigurePrinter(config)

	assert.Equal(t, &buf, config.Err)
}

func TestWithOut_WithErr_Combined(t *testing.T) {
	t.Parallel()

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	withOut := WithOut{Out: &outBuf}
	withErr := WithErr{Err: &errBuf}
	config := &PrinterConfig{}

	withOut.ConfigurePrinter(config)
	withErr.ConfigurePrinter(config)

	assert.Equal(t, &outBuf, config.Out)
	assert.Equal(t, &errBuf, config.Err)
}

func TestWithOut_NilWriter(t *testing.T) {
	t.Parallel()

	withOut := WithOut{Out: nil}
	config := &PrinterConfig{}

	withOut.ConfigurePrinter(config)

	assert.Nil(t, config.Out)
}

func TestWithErr_NilWriter(t *testing.T) {
	t.Parallel()

	withErr := WithErr{Err: nil}
	config := &PrinterConfig{}

	withErr.ConfigurePrinter(config)

	assert.Nil(t, config.Err)
}
