package integration_test

import (
	"context"
	"log"
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

	ctx := context.Background()
	addonOperator := &addonsv1alpha1.AddonOperator{}
	if err := integration.Client.Get(ctx, client.ObjectKey{
		Name: addonsv1alpha1.DefaultAddonOperatorName,
	}, addonOperator); err != nil {
		s.T().Fatalf("get AddonOperator object: %v", err)
	}

	addonOperator.Spec.OCM = &addonsv1alpha1.AddonOperatorOCM{
		Endpoint: integration.OCMAPIEndpoint,
		Secret: addonsv1alpha1.ClusterSecretReference{
			Name:      "api-mock",
			Namespace: "api-mock",
		},
	}
	if err := integration.Client.Update(ctx, addonOperator); err != nil {
		s.T().Fatalf("patch AddonOperator object: %v", err)
	}
}

func (s *integrationTestSuite) TearDownSuite() {
	if !testing.Short() {
		s.Teardown()
	}

	if err := integration.PrintPodStatusAndLogs("addon-operator"); err != nil {
		log.Fatal(err)
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
	// Run kube-apiserver proxy during tests
	apiProxyCloseCh := make(chan struct{})
	defer close(apiProxyCloseCh)
	if err := integration.RunAPIServerProxy(apiProxyCloseCh); err != nil {
		t.Fatal(err)
	}

	// does not support parallel test runs
	suite.Run(t, new(integrationTestSuite))
}
