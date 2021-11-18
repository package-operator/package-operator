package integration_test

import (
	"context"
	"time"

	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) Teardown() {
	ctx := context.Background()
	objs := integration.LoadObjectsFromDeploymentFiles(s.T())

	// reverse object order for de-install
	for i, j := 0, len(objs)-1; i < j; i, j = i+1, j-1 {
		objs[i], objs[j] = objs[j], objs[i]
	}

	// Delete all objects to teardown the Addon Operator
	for _, obj := range objs {
		err := integration.Client.Delete(ctx, &obj)
		s.Require().NoError(err)

		s.T().Log("deleted: ", obj.GroupVersionKind().String(),
			obj.GetNamespace()+"/"+obj.GetName())
	}

	s.Run("everything is gone", func() {
		for _, obj := range objs {
			// Namespaces can take a long time to be cleaned up and
			// there is no need to be specific about the object kind here
			s.Assert().NoError(integration.WaitToBeGone(s.T(), 2*time.Minute, &obj))
		}
	})
}
