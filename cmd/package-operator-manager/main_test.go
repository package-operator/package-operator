package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/cmd/package-operator-manager/components"
)

func TestVersionPrinting(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}

	o := components.Options{PrintVersion: buf}

	require.NoError(t, run(o))

	require.Contains(t, buf.String(), "package-operator.run/cmd/package-operator-manager")
}
