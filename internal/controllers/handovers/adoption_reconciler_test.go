package handovers

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

func Test_negativeLabelKeySelectorFromLabels(t *testing.T) {
	s, err := negativeLabelKeySelectorFromLabels(map[string]string{
		"label": "false",
	})
	require.NoError(t, err)

	assert.False(t, s.Matches(labels.Set{"label": "banana"}))
	assert.True(t, s.Matches(labels.Set{"somethingelse": "banana"}))
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

	r := newAdoptionReconciler(clientMock, clientMock)

	ctx := context.Background()
	res, err := r.Reconcile(ctx, &GenericClusterHandover{
		ClusterHandover: coordinationv1alpha1.ClusterHandover{
			Spec: coordinationv1alpha1.ClusterHandoverSpec{
				Strategy: coordinationv1alpha1.HandoverStrategy{
					Type: coordinationv1alpha1.HandoverStrategyRelabel,
					Relabel: &coordinationv1alpha1.HandoverStrategyRelabelSpec{
						LabelKey:     "operator-version",
						InitialValue: "v1",
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
