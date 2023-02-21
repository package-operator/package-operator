package adoption

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	coordinationv1alpha1 "package-operator.run/apis/coordination/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
	"package-operator.run/package-operator/internal/testutil/dynamiccachemocks"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := coordinationv1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestClusterAdoptionController(t *testing.T) {
	c := testutil.NewClient()
	dc := &dynamiccachemocks.DynamicCacheMock{}

	dc.
		On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	c.On("Get",
		mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterAdoption"), mock.Anything).
		Return(nil)

	c.On("Patch",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterAdoption"),
		mock.Anything, mock.Anything).
		Return(nil)

	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterAdoption"),
		mock.Anything).
		Return(nil)

	ac := NewClusterAdoptionController(
		c, logr.Discard(), dc, testScheme)

	ctx := context.Background()
	res, err := ac.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

func TestClusterAdoptionController_deletion(t *testing.T) {
	c := testutil.NewClient()
	dc := &dynamiccachemocks.DynamicCacheMock{}

	dc.
		On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	dc.
		On("Free", mock.Anything, mock.Anything).
		Return(nil)

	c.On("Get",
		mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterAdoption"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*coordinationv1alpha1.ClusterAdoption)
			now := metav1.Now()
			*obj = coordinationv1alpha1.ClusterAdoption{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			}
		}).
		Return(nil)

	c.On("Patch",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterAdoption"),
		mock.Anything, mock.Anything).
		Return(nil)

	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterAdoption"),
		mock.Anything).
		Return(nil)

	ac := NewClusterAdoptionController(
		c, logr.Discard(), dc, testScheme)

	ctx := context.Background()
	res, err := ac.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

func TestAdoptionController(t *testing.T) {
	c := testutil.NewClient()
	dc := &dynamiccachemocks.DynamicCacheMock{}

	dc.
		On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	c.On("Get",
		mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.Adoption"), mock.Anything).
		Return(nil)

	c.On("Patch",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Adoption"),
		mock.Anything, mock.Anything).
		Return(nil)

	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Adoption"),
		mock.Anything).
		Return(nil)

	ac := NewAdoptionController(
		c, logr.Discard(), dc, testScheme)

	ctx := context.Background()
	res, err := ac.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

func TestAdoptionController_deletion(t *testing.T) {
	c := testutil.NewClient()
	dc := &dynamiccachemocks.DynamicCacheMock{}

	dc.
		On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	dc.
		On("Free", mock.Anything, mock.Anything).
		Return(nil)

	c.On("Get",
		mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.Adoption"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(2).(*coordinationv1alpha1.Adoption)
			now := metav1.Now()
			*obj = coordinationv1alpha1.Adoption{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &now,
				},
			}
		}).
		Return(nil)

	c.On("Patch",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Adoption"),
		mock.Anything, mock.Anything).
		Return(nil)

	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Adoption"),
		mock.Anything).
		Return(nil)

	ac := NewAdoptionController(
		c, logr.Discard(), dc, testScheme)

	ctx := context.Background()
	res, err := ac.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}
