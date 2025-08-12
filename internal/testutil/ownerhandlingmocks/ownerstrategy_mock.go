package ownerhandlingmocks

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"pkg.package-operator.run/boxcutter/ownerhandling"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

var _ ownerhandling.OwnerStrategy = (*OwnerStrategyMock)(nil)

type OwnerStrategyMock struct {
	mock.Mock
}

func (m *OwnerStrategyMock) GetController(obj metav1.Object) (metav1.OwnerReference, bool) {
	args := m.Called(obj)
	return args.Get(0).(metav1.OwnerReference), args.Bool(1)
}

func (m *OwnerStrategyMock) IsController(owner, obj metav1.Object) bool {
	args := m.Called(owner, obj)
	return args.Bool(0)
}

func (m *OwnerStrategyMock) IsOwner(owner, obj metav1.Object) bool {
	args := m.Called(owner, obj)
	return args.Bool(0)
}

func (m *OwnerStrategyMock) RemoveOwner(owner, obj metav1.Object) {
	m.Called(owner, obj)
}

func (m *OwnerStrategyMock) ReleaseController(obj metav1.Object) {
	m.Called(obj)
}

func (m *OwnerStrategyMock) SetOwnerReference(owner, obj metav1.Object) error {
	args := m.Called(owner, obj)
	return args.Error(0)
}

func (m *OwnerStrategyMock) SetControllerReference(owner, obj metav1.Object) error {
	args := m.Called(owner, obj)
	return args.Error(0)
}

func (m *OwnerStrategyMock) EnqueueRequestForOwner(
	ownerType client.Object, mapper meta.RESTMapper, isController bool,
) handler.EventHandler {
	args := m.Called(ownerType, mapper, isController)
	return args.Get(0).(handler.EventHandler)
}

func (m *OwnerStrategyMock) CopyOwnerReferences(objA, objB metav1.Object) {
	_ = m.Called(objA, objB)
}
