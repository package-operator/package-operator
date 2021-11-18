package integration_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
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

func TestIntegration(t *testing.T) {
	suite.Run(t, new(integrationTestSuite))
}
