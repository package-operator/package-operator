package teardown

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/addon-operator/e2e"
)

func TestTeardown(t *testing.T) {
	ctx := context.Background()
	objs := e2e.LoadObjectsFromDeploymentFiles(t)

	// reverse object order for de-install
	for i, j := 0, len(objs)-1; i < j; i, j = i+1, j-1 {
		objs[i], objs[j] = objs[j], objs[i]
	}

	// Delete all objects to teardown the Addon Operator
	for _, obj := range objs {
		err := e2e.Client.Delete(ctx, &obj)
		require.NoError(t, err)

		t.Log("deleted: ", obj.GroupVersionKind().String(),
			obj.GetNamespace()+"/"+obj.GetName())
	}

	t.Run("everything is gone", func(t *testing.T) {
		for _, obj := range objs {
			// Namespaces can take a long time to be cleaned up and
			// there is no need to be specific about the object kind here
			assert.NoError(t, e2e.WaitToBeGone(t, 2*time.Minute, &obj))
		}
	})
}
