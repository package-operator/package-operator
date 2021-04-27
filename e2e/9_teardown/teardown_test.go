package teardown

import (
	"context"
	"testing"

	"github.com/openshift/addon-operator/e2e"
	"github.com/stretchr/testify/require"
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
}
