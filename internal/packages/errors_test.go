package packages

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestNewInvalidAggregate(t *testing.T) {
	t.Parallel()

	err1 := &InvalidError{
		Violations: []Violation{
			{Reason: "broken"},
		},
	}
	err2 := &InvalidError{
		Violations: []Violation{
			{Reason: "on fire"},
		},
	}

	err := NewInvalidAggregate(err1, err2, nil)

	var aggregatedErr *InvalidError
	require.True(t, errors.As(err, &aggregatedErr))
	assert.Equal(t, []Violation{
		{Reason: "broken"},
		{Reason: "on fire"},
	}, aggregatedErr.Violations)
}

func TestInvalidError(t *testing.T) {
	t.Parallel()

	err1 := &InvalidError{
		Violations: []Violation{
			{Reason: "broken"},
			{Reason: "on fire", Details: "hot stuff!"},
		},
	}
	assert.Equal(t, `Package validation errors:
- broken
- on fire:
  hot stuff!`, err1.Error())
}

func TestViolation(t *testing.T) {
	t.Parallel()

	t.Run("with location", func(t *testing.T) {
		t.Parallel()

		v := Violation{
			Reason:  "broken",
			Details: "on fire",
			Location: &ViolationLocation{
				Path: "hot_stuff/on_fire.yaml",
			},
		}

		assert.Equal(t, "broken in hot_stuff/on_fire.yaml:\non fire", v.String())
	})

	t.Run("without location", func(t *testing.T) {
		t.Parallel()

		v := Violation{Reason: "broken", Details: "on fire"}

		assert.Equal(t, "broken:\non fire", v.String())
	})
}

func TestViolationLocation(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		var vl *ViolationLocation
		assert.Equal(t, "", vl.String())
	})

	t.Run("just path", func(t *testing.T) {
		t.Parallel()

		vl := &ViolationLocation{Path: "test/234.yaml"}
		assert.Equal(t, "test/234.yaml", vl.String())
	})

	t.Run("with doc index", func(t *testing.T) {
		t.Parallel()

		vl := &ViolationLocation{Path: "test/234.yaml", DocumentIndex: ptr.To(3)}
		assert.Equal(t, "test/234.yaml#3", vl.String())
	})
}
