package controllers

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsExternalResourceNotFound(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		Error     error
		Assertion assert.BoolAssertionFunc
	}{
		"nil": {
			Error:     nil,
			Assertion: assert.False,
		},
		"external resource not found error": {
			Error:     NewExternalResourceNotFoundError(nil),
			Assertion: assert.True,
		},
		"wrapped external resource not found error": {
			Error:     fmt.Errorf("%w", NewExternalResourceNotFoundError(nil)),
			Assertion: assert.True,
		},
		"io error": {
			Error:     io.EOF,
			Assertion: assert.False,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.Assertion(t, IsExternalResourceNotFound(tc.Error))
		})
	}
}

func TestPhaseReconcilerErrorInterfaces(t *testing.T) {
	t.Parallel()

	require.Implements(t, new(error), new(PhaseReconcilerError))
	require.Implements(t, new(ControllerError), new(PhaseReconcilerError))
}
