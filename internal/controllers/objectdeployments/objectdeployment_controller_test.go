package objectdeployments

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestObjectDeployment_Reconciler(t *testing.T) {
	t.Parallel()

	clientMock := testutil.NewClient()
	c := NewObjectDeploymentController(
		clientMock, ctrl.Log.WithName("object deployment test"), testScheme)

	odName := "test-od"
	od := &corev1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: odName,
		},
	}

	clientMock.
		On("Get", mock.Anything, mock.Anything, mock.AnythingOfType("*corev1alpha1.ObjectDeployment"), mock.Anything).
		Run(func(args mock.Arguments) {
			obj := args.Get(1).(*corev1alpha1.ObjectDeployment)
			od.DeepCopyInto(obj)
		}).
		Return(nil)

	ctx := context.Background()
	res, err := c.Reconcile(ctx, ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(od),
	})

	require.Error(t, err)
	assert.True(t, res.IsZero())
}
