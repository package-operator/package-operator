package discoveryclientmock

import (
	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/version"
)

type DiscoveryClientMock struct {
	mock.Mock
}

func (c *DiscoveryClientMock) ServerVersion() (*version.Info, error) {
	args := c.Called()
	return args.Get(0).(*version.Info), args.Error(1)
}

func (c *DiscoveryClientMock) OpenAPISchema() (*openapi_v2.Document, error) {
	// TODO
	return nil, nil
}
