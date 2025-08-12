package discoveryclientmock

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/openapi"
)

type DiscoveryClientMock struct {
	mock.Mock
}

func (c *DiscoveryClientMock) ServerVersion() (*version.Info, error) {
	args := c.Called()
	return args.Get(0).(*version.Info), args.Error(1)
}

func (c *DiscoveryClientMock) OpenAPIV3() openapi.Client {
	args := c.Called(0)
	return args.Get(0).(openapi.Client)
}
