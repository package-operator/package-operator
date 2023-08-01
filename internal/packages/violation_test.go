package packages_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages"
)

func TestViolationErrorDefault(t *testing.T) {
	t.Parallel()

	v := packages.ViolationError{}
	require.Equal(t, string(packages.ViolationReasonUnknown), v.Error())
}

func TestViolationErrorReason(t *testing.T) {
	t.Parallel()

	v := packages.ViolationError{Reason: packages.ViolationReason("cheese reason")}
	require.EqualError(t, v, "cheese reason")
}

func TestViolationErrorDetail(t *testing.T) {
	t.Parallel()

	v := packages.ViolationError{Reason: packages.ViolationReason("cheese reason"), Details: "zoom 200x"}
	require.EqualError(t, v, "cheese reason: zoom 200x")
}

func TestViolationErrorPath(t *testing.T) {
	t.Parallel()

	v := packages.ViolationError{Reason: packages.ViolationReason("cheese reason"), Path: "a/b", Index: packages.Index(4)}
	require.EqualError(t, v, "cheese reason in a/b idx 4")
}

func TestViolationErrorDetailPath(t *testing.T) {
	t.Parallel()

	v := packages.ViolationError{Reason: packages.ViolationReason("cheese reason"), Details: "zoom 200x", Path: "a/b"}
	require.EqualError(t, v, "cheese reason in a/b: zoom 200x")
}
