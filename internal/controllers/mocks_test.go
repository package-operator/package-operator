package controllers

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil/ownerhandlingmocks"
)

type previousOwnerMock struct {
	mock.Mock
}

func (m *previousOwnerMock) ClientObject() client.Object {
	args := m.Called()
	return args.Get(0).(client.Object)
}

func (m *previousOwnerMock) GetPrevious() []corev1alpha1.PreviousRevisionReference {
	args := m.Called()
	return args.Get(0).([]corev1alpha1.PreviousRevisionReference)
}

type ownerStrategyMock = ownerhandlingmocks.OwnerStrategyMock

type previousObjectSetMock struct {
	mock.Mock
}

func (m *previousObjectSetMock) ClientObject() client.Object {
	args := m.Called()
	return args.Get(0).(client.Object)
}

func (m *previousObjectSetMock) GetRemotePhases() []corev1alpha1.RemotePhaseReference {
	args := m.Called()
	return args.Get(0).([]corev1alpha1.RemotePhaseReference)
}

type previousObjectSetMockFactory struct {
	mock.Mock
}

func (m *previousObjectSetMockFactory) New(*runtime.Scheme) PreviousObjectSet {
	args := m.Called()
	return args.Get(0).(PreviousObjectSet)
}
