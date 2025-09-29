package objectsets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestRevisionReconciler_SetStatusFromSpec(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()
	testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r := &revisionReconciler{
		scheme:       testScheme,
		newObjectSet: adapters.NewObjectSet,
		client:       testClient,
	}

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "xxx",
			},
			Spec: corev1alpha1.ObjectSetSpec{
				Revision: 42,
			},
		},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)

	assert.True(t, res.IsZero(), "unexpected requeue")
	assert.Equal(t, objectSet.Spec.Revision, objectSet.Status.Revision) //nolint:staticcheck
}
