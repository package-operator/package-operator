package preflight

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"package-operator.run/package-operator/internal/testutil"
)

var errTest = errors.New("explosion")

func TestDryRun(t *testing.T) {
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

	dr := NewDryRun(c)
	v, err := dr.Check(context.Background(), obj, obj)
	require.Error(t, err)
	assert.Len(t, v, 0)
	// MUST create an internal DeepCopy or the DryRun hook may have changed the object.
	assert.NotSame(t, objCalled, obj)
}

func TestDryRun_alreadyExists(t *testing.T) {
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

	dr := NewDryRun(c)
	v, err := dr.Check(context.Background(), obj, obj)
	require.NoError(t, err)
	assert.Len(t, v, 0)
	// MUST create an internal DeepCopy or the DryRun hook may have changed the object.
	assert.NotSame(t, objCalled, obj)
}

func TestDryRun_emptyreason(t *testing.T) {
	c := testutil.NewClient()

	e := &k8serrors.StatusError{
		ErrStatus: v1.Status{Reason: "", Message: "cheese, also failed to create typed patch object, also more cheese"},
	}
	c.On("Patch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(e)

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("test-ns")
	obj.SetKind("Hans")

	dr := NewDryRun(c)
	v, err := dr.Check(context.Background(), obj, obj)
	require.NoError(t, err)
	require.Len(t, v, 1)
	require.Contains(t, v[0].Error, e.ErrStatus.Message)
}
