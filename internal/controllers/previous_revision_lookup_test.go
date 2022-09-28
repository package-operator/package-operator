package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func TestPreviousRevisionLookup(t *testing.T) {
	factory := &previousObjectSetMockFactory{}
	clientMock := testutil.NewClient()

	rl := NewPreviousRevisionLookup(nil, factory.New, clientMock)

	ctx := context.Background()

	prev := &previousObjectSetMock{}
	prev.
		On("ClientObject").
		Return(&corev1.ConfigMap{})

	factory.
		On("New", mock.Anything).
		Return(prev)

	owner := &previousOwnerMock{}
	owner.
		On("GetPrevious").
		Return([]corev1alpha1.PreviousRevisionReference{
			{
				Name: "test1",
			},
		})
	owner.
		On("ClientObject").
		Return(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test-ns",
			},
		})

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	previous, err := rl.Lookup(ctx, owner)
	require.NoError(t, err)
	if assert.Len(t, previous, 1) {
		assert.Equal(t, prev, previous[0])
	}

	clientMock.AssertCalled(t, "Get", mock.Anything, client.ObjectKey{
		Name:      "test1",
		Namespace: "test-ns",
	}, prev.ClientObject(), mock.Anything)
}
