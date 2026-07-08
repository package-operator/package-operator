package packagetypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestViolationErrorDefault(t *testing.T) {
	t.Parallel()

	v := ViolationError{}
	require.Equal(t, string(ViolationReasonUnknown), v.Error())
}

func TestViolationErrorReason(t *testing.T) {
	t.Parallel()

	v := ViolationError{Reason: ViolationReason("cheese reason")}
	require.EqualError(t, v, "cheese reason")
}

func TestViolationErrorDetail(t *testing.T) {
	t.Parallel()

	v := ViolationError{Reason: ViolationReason("cheese reason"), Details: "zoom 200x"}
	require.EqualError(t, v, "cheese reason: zoom 200x")
}

func TestViolationErrorPath(t *testing.T) {
	t.Parallel()

	v := ViolationError{Reason: ViolationReason("cheese reason"), Path: "a/b", Index: new(4)}
	require.EqualError(t, v, "cheese reason in a/b idx 4")
}

func TestViolationErrorDetailPath(t *testing.T) {
	t.Parallel()

	v := ViolationError{Reason: ViolationReason("cheese reason"), Details: "zoom 200x", Path: "a/b"}
	require.EqualError(t, v, "cheese reason in a/b: zoom 200x")
}

func TestViolationErrorDetailPathSubject(t *testing.T) {
	t.Parallel()

	v := ViolationError{
		Reason:  ViolationReason("cheese reason"),
		Details: "zoom 200x",
		Path:    "a/b", Subject: "yaml: test\n",
	}
	require.EqualError(t, v, "cheese reason in a/b: zoom 200x\nyaml: test")
}
