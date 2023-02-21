package adoption

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	coordinationv1alpha1 "package-operator.run/apis/coordination/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func TestStaticStrategyReconciler_noop(t *testing.T) {
	clientMock := testutil.NewClient()

	r := newStaticAdoptionReconciler(clientMock, clientMock)

	ctx := context.Background()
	res, err := r.Reconcile(ctx, &GenericAdoption{})
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

func TestStaticStrategyReconciler_setsLabels(t *testing.T) {
	clientMock := testutil.NewClient()

	clientMock.
		On("List", mock.Anything,
			mock.AnythingOfType("*unstructured.UnstructuredList"), mock.Anything).
		Run(func(args mock.Arguments) {
			l := args.Get(1).(*unstructured.UnstructuredList)
			l.Items = []unstructured.Unstructured{
				{},
			}
		}).
		Return(nil)

	var updatedObj *unstructured.Unstructured
	clientMock.
		On("Update", mock.Anything,
			mock.AnythingOfType("*unstructured.Unstructured"), mock.Anything).
		Run(func(args mock.Arguments) {
			updatedObj = args.Get(1).(*unstructured.Unstructured)
		}).
		Return(nil)

	r := newStaticAdoptionReconciler(clientMock, clientMock)

	ctx := context.Background()
	res, err := r.Reconcile(ctx, &GenericAdoption{
		Adoption: coordinationv1alpha1.Adoption{
			Spec: coordinationv1alpha1.AdoptionSpec{
				Strategy: coordinationv1alpha1.AdoptionStrategy{
					Type: coordinationv1alpha1.AdoptionStrategyStatic,
					Static: &coordinationv1alpha1.AdoptionStrategyStaticSpec{
						Labels: map[string]string{
							"operator-version": "v1",
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	if clientMock.AssertCalled(t, "Update", mock.Anything, mock.AnythingOfType("*unstructured.Unstructured"), mock.Anything) {
		assert.Equal(t, map[string]string{
			"operator-version": "v1",
		}, updatedObj.GetLabels())
	}
}

func Test_negativeLabelKeySelectorFromLabels(t *testing.T) {
	s, err := negativeLabelKeySelectorFromLabels(map[string]string{
		"label": "false",
	})
	require.NoError(t, err)

	assert.False(t, s.Matches(labels.Set{"label": "banana"}))
	assert.True(t, s.Matches(labels.Set{"somethingelse": "banana"}))
}
