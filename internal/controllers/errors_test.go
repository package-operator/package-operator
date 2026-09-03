package controllers

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.package-operator.run/boxcutter/machinery"
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

func TestIsAdoptionRefusedError(t *testing.T) {
	t.Parallel()

	collisionErr := machinery.NewCreateCollisionError(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
		"already exists",
	)

	for name, tc := range map[string]struct {
		err       error
		assertion assert.BoolAssertionFunc
	}{
		"nil": {
			err:       nil,
			assertion: assert.False,
		},
		"create collision error": {
			err:       collisionErr,
			assertion: assert.True,
		},
		"wrapped create collision error": {
			err:       fmt.Errorf("reconcile failed: %w", collisionErr),
			assertion: assert.True,
		},
		"other error": {
			err:       io.EOF,
			assertion: assert.False,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.assertion(t, IsAdoptionRefusedError(tc.err))
		})
	}
}

func TestPhaseReconcilerErrorInterfaces(t *testing.T) {
	t.Parallel()

	require.Implements(t, new(error), new(PhaseReconcilerError))
	require.Implements(t, new(ControllerError), new(PhaseReconcilerError))
}
