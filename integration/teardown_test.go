package integration_test

import (
	"context"
	"time"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func (s *integrationTestSuite) Teardown() {
	ctx := context.Background()

	// assert that all addons are gone before teardown
	addonList := &addonsv1alpha1.AddonList{}
	err := integration.Client.List(ctx, addonList)
	s.Assert().NoError(err)
	addonNames := []string{}
	for _, a := range addonList.Items {
		addonNames = append(addonNames, a.GetName())
	}
	s.Assert().Len(addonNames, 0, "expected all Addons to be gone before teardown, but some still exist")

	objs := integration.LoadObjectsFromDeploymentFiles(s.T())

	// reverse object order for de-install
	for i, j := 0, len(objs)-1; i < j; i, j = i+1, j-1 {
		objs[i], objs[j] = objs[j], objs[i]
	}

	// Delete all objects to teardown the Addon Operator
	for _, obj := range objs {
		o := obj
		err := integration.Client.Delete(ctx, &o)
		s.Assert().NoError(err)

		s.T().Log("deleted: ", o.GroupVersionKind().String(),
			o.GetNamespace()+"/"+o.GetName())
	}

	s.Run("everything is gone", func() {
		for _, obj := range objs {
			// Namespaces can take a long time to be cleaned up and
			// there is no need to be specific about the object kind here
			o := obj
			s.Assert().NoError(integration.WaitToBeGone(s.T(), 10*time.Minute, &o))
		}
	})
}
