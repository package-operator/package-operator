package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

type integrationTestSuite struct {
	suite.Suite
}

func (s *integrationTestSuite) SetupSuite() {
	if !testing.Short() {
		s.Setup()
	}
}

func (s *integrationTestSuite) TearDownSuite() {
	if !testing.Short() {
		s.Teardown()
	}
}

func (s *integrationTestSuite) addonCleanup(addon *addonsv1alpha1.Addon,
	ctx context.Context) {
	// delete Addon
	err := integration.Client.Delete(ctx, addon, client.PropagationPolicy("Foreground"))
	s.Require().NoError(err, "delete Addon: %v", addon)

	// wait until Addon is gone
	err = integration.WaitToBeGone(s.T(), defaultAddonDeletionTimeout, addon)
	s.Require().NoError(err, "wait for Addon to be deleted")
}

func TestIntegration(t *testing.T) {
	// does not support parallel test runs
	suite.Run(t, new(integrationTestSuite))
}
