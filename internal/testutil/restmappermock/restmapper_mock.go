package restmappermock

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type RestMapperMock struct {
	mock.Mock
}

var _ meta.RESTMapper = (*RestMapperMock)(nil)

func (m *RestMapperMock) KindFor(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	args := m.Called()

	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *RestMapperMock) KindsFor(schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	args := m.Called()

	return args.Get(0).([]schema.GroupVersionKind), args.Error(1)
}

func (m *RestMapperMock) ResourceFor(schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	args := m.Called()

	return args.Get(0).(schema.GroupVersionResource), args.Error(1)
}

func (m *RestMapperMock) ResourcesFor(schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	args := m.Called()

	return args.Get(0).([]schema.GroupVersionResource), args.Error(1)
}

func (m *RestMapperMock) ResourceSingularizer(string) (string, error) {
	args := m.Called()

	return args.String(0), args.Error(1)
}

func (m *RestMapperMock) RESTMappings(schema.GroupKind, ...string) ([]*meta.RESTMapping, error) {
	args := m.Called()

	return args.Get(0).([]*meta.RESTMapping), args.Error(1)
}

func (m *RestMapperMock) RESTMapping(schema.GroupKind, ...string) (*meta.RESTMapping, error) {
	args := m.Called()

	return args.Get(0).(*meta.RESTMapping), args.Error(1)
}
