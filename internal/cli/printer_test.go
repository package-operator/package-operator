package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/cmd"
)

func TestPrinter_PrintfOut(t *testing.T) {
	t.Parallel()

	const expected = "test"

	var out bytes.Buffer

	printer := NewPrinter(
		WithOut{Out: &out},
	)

	require.NoError(t, printer.PrintfOut(expected))

	assert.Equal(t, expected, out.String())
}

func TestPrinter_PrintfErr(t *testing.T) {
	t.Parallel()

	const expected = "test"

	var err bytes.Buffer

	printer := NewPrinter(
		WithErr{Err: &err},
	)

	require.NoError(t, printer.PrintfErr(expected))

	assert.Equal(t, expected, err.String())
}

func TestPrinter_PrintTable(t *testing.T) {
	t.Parallel()

	const expected = "One  Two  Three\n1    2    3\n\n"

	table := cmd.NewDefaultTable(
		cmd.WithHeaders{
			"One", "Two", "Three",
		},
	)

	table.AddRow(
		cmd.Field{
			Name:  "One",
			Value: 1,
		},
		cmd.Field{
			Name:  "Two",
			Value: 2,
		},
		cmd.Field{
			Name:  "Three",
			Value: 3,
		},
	)

	var out bytes.Buffer

	printer := NewPrinter(
		WithOut{Out: &out},
	)

	require.NoError(t, printer.PrintTable(table))

	assert.Equal(t, expected, out.String())
}
