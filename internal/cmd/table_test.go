package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTable_Rows(t *testing.T) {
	t.Parallel()

	expectedRows := [][]Field{
		{
			Field{
				Name:  "One",
				Value: 1,
			},
		},
	}

	table := NewDefaultTable(
		WithHeaders{"One"},
	)
	table.AddRow(
		Field{
			Name:  "One",
			Value: 1,
		},
	)
	table.AddRow(
		Field{
			Name:  "Two",
			Value: 2,
		},
	)

	assert.Equal(t, expectedRows, table.Rows())
}
