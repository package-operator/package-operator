package ownerhandlingmocks

import (
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type OwnerStrategyMock struct {
	mock.Mock
}

func (m *OwnerStrategyMock) OwnerPatch(obj metav1.Object) ([]byte, error) {
	args := m.Called(obj)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *OwnerStrategyMock) HasController(obj metav1.Object) bool {
	args := m.Called(obj)
	return args.Bool(0)
}

func (m *OwnerStrategyMock) IsController(owner, obj metav1.Object) bool {
	args := m.Called(owner, obj)
	return args.Bool(0)
}

func (m *OwnerStrategyMock) RemoveOwner(owner, obj metav1.Object) {
	m.Called(owner, obj)
}

func (m *OwnerStrategyMock) ReleaseController(obj metav1.Object) {
	m.Called(obj)
}

func (m *OwnerStrategyMock) SetControllerReference(owner, obj metav1.Object) error {
	args := m.Called(owner, obj)
	return args.Error(0)
}

func (m *OwnerStrategyMock) EnqueueRequestForOwner(ownerType client.Object, isController bool) handler.EventHandler {
	args := m.Called(ownerType, isController)
	return args.Get(0).(handler.EventHandler)
}
