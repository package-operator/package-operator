package objectdeployments

import (
	"testing"

	"github.com/stretchr/testify/require"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/utils"
)

func TestHashReconciler(t *testing.T) {
	t.Parallel()

	t.Run("get correct hash", func(t *testing.T) {
		t.Parallel()

		testClient := testutil.NewClient()

		hr := hashReconciler{
			client: testClient,
		}

		ctx := t.Context()

		objectSetDeployment := &genericObjectSetDeploymentMock{}
		objectSetDeployment.On("GetObjectSetTemplate").Return(corev1alpha1.ObjectSetTemplate{})
		objectSetDeployment.On("GetStatusCollisionCount").Return(1)

		hash := utils.ComputeFNV32Hash(
			objectSetDeployment.GetObjectSetTemplate(),
			objectSetDeployment.GetStatusCollisionCount(),
		)
		objectSetDeployment.On("SetStatusTemplateHash", hash)

		_, err := hr.Reconcile(ctx, objectSetDeployment)
		require.NoError(t, err)
		objectSetDeployment.AssertExpectations(t)
	})
}
