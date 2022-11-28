package packagestructure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestNewInvalidAggregate(t *testing.T) {
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

	aggregatedErr := NewInvalidAggregate(err1, err2, nil)
	assert.Equal(t, []Violation{
		{Reason: "broken"},
		{Reason: "on fire"},
	}, aggregatedErr.Violations)
}

func TestInvalidError(t *testing.T) {
	err1 := &InvalidError{
		Violations: []Violation{
			{Reason: "broken"},
			{Reason: "on fire"},
		},
	}
	assert.Equal(t, `Package validation errors:
- broken
- on fire
`, err1.Error())
}

func TestViolation(t *testing.T) {
	t.Run("with location", func(t *testing.T) {
		v := Violation{
			Reason:  "broken",
			Details: "on fire",
			Location: &ViolationLocation{
				Path: "hot_stuff/on_fire.yaml",
			},
		}

		assert.Equal(t,
			"broken on fire in hot_stuff/on_fire.yaml", v.String())
	})

	t.Run("without location", func(t *testing.T) {
		v := Violation{
			Reason:  "broken",
			Details: "on fire",
		}

		assert.Equal(t,
			"broken on fire", v.String())
	})
}

func TestViolationLocation(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var vl *ViolationLocation
		assert.Equal(t, "", vl.String())
	})

	t.Run("just path", func(t *testing.T) {
		vl := &ViolationLocation{
			Path: "test/234.yaml",
		}
		assert.Equal(t, "test/234.yaml", vl.String())
	})

	t.Run("with doc index", func(t *testing.T) {
		vl := &ViolationLocation{
			Path:          "test/234.yaml",
			DocumentIndex: pointer.Int(3),
		}
		assert.Equal(t, "test/234.yaml#3", vl.String())
	})
}
