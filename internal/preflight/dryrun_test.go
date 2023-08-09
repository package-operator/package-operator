package preflight_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"package-operator.run/internal/preflight"
	"package-operator.run/internal/testutil"
)

var errTest = errors.New("explosion")

func TestDryRun(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()

	var objCalled *unstructured.Unstructured
	c.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			objCalled = args.Get(1).(*unstructured.Unstructured)
		}).
		Return(errTest)

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("test-ns")
	obj.SetKind("Hans")

	dr := preflight.NewDryRun(c)
	v, err := dr.Check(context.Background(), obj, obj)
	require.Error(t, err)
	assert.Len(t, v, 0)
	// MUST create an internal DeepCopy or the DryRun hook may have changed the object.
	assert.NotSame(t, objCalled, obj)
}

func TestDryRunViolations(t *testing.T) {
	t.Parallel()

	reasons := []metav1.StatusReason{
		metav1.StatusReasonUnauthorized,
		metav1.StatusReasonForbidden,
		metav1.StatusReasonAlreadyExists,
		metav1.StatusReasonConflict,
		metav1.StatusReasonInvalid,
		metav1.StatusReasonBadRequest,
		metav1.StatusReasonMethodNotAllowed,
		metav1.StatusReasonRequestEntityTooLarge,
		metav1.StatusReasonUnsupportedMediaType,
		metav1.StatusReasonNotAcceptable,
	}

	for i := range reasons {
		reason := reasons[i]
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()
			c := testutil.NewClient()

			c.
				On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&k8serrors.StatusError{ErrStatus: metav1.Status{Reason: reason}})

			obj := &unstructured.Unstructured{}
			obj.SetName("test")
			obj.SetNamespace("test-ns")
			obj.SetKind("Hans")

			dr := preflight.NewDryRun(c)
			v, err := dr.Check(context.Background(), obj, obj)
			require.NoError(t, err)
			assert.Len(t, v, 1)
		})
	}
}

func TestDryRun_alreadyExists(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()

	var objCalled *unstructured.Unstructured
	c.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			objCalled = args.Get(1).(*unstructured.Unstructured)
		}).
		Return(k8serrors.NewAlreadyExists(schema.GroupResource{}, ""))
	c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("test-ns")
	obj.SetKind("Hans")

	dr := preflight.NewDryRun(c)
	v, err := dr.Check(context.Background(), obj, obj)
	require.NoError(t, err)
	assert.Len(t, v, 0)
	// MUST create an internal DeepCopy or the DryRun hook may have changed the object.
	assert.NotSame(t, objCalled, obj)
}

func TestDryRun_notFround(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()

	c.
		On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&k8serrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})
	c.
		On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&k8serrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("test-ns")
	obj.SetKind("Hans")

	dr := preflight.NewDryRun(c)
	v, err := dr.Check(context.Background(), obj, obj)
	require.NoError(t, err)
	assert.Len(t, v, 1)
}

func TestDryRun_emptyreason(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()

	e := &k8serrors.StatusError{
		ErrStatus: metav1.Status{Reason: "", Message: "cheese, also failed to create typed patch object, also more cheese"},
	}
	c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(e)

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("test-ns")
	obj.SetKind("Hans")

	dr := preflight.NewDryRun(c)
	v, err := dr.Check(context.Background(), obj, obj)
	require.NoError(t, err)
	require.Len(t, v, 1)
	require.Contains(t, v[0].Error, e.ErrStatus.Message)
}
