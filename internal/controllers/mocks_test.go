package controllers

import (
	"context"

	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
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

type ownerStrategyMock struct {
	mock.Mock
}

func (m *ownerStrategyMock) IsController(owner, obj metav1.Object) bool {
	args := m.Called(owner, obj)
	return args.Bool(0)
}

func (m *ownerStrategyMock) RemoveOwner(owner, obj metav1.Object) {
	m.Called(owner, obj)
}

func (m *ownerStrategyMock) ReleaseController(obj metav1.Object) {
	m.Called(obj)
}

func (m *ownerStrategyMock) SetControllerReference(owner, obj metav1.Object) error {
	args := m.Called(owner, obj)
	return args.Error(0)
}

type phaseObjectOwnerMock struct {
	mock.Mock
}

func (m *phaseObjectOwnerMock) ClientObject() client.Object {
	args := m.Called()
	return args.Get(0).(client.Object)
}

func (m *phaseObjectOwnerMock) GetRevision() int64 {
	args := m.Called()
	return args.Get(0).(int64)
}

func (m *phaseObjectOwnerMock) IsPaused() bool {
	args := m.Called()
	return args.Bool(0)
}

type dynamicCacheMock struct {
	testutil.CtrlClient
}

func (c *dynamicCacheMock) Watch(
	ctx context.Context, owner client.Object, obj runtime.Object,
) error {
	args := c.Called(ctx, owner, obj)
	return args.Error(0)
}

type adoptionCheckerMock struct {
	mock.Mock
}

func (m *adoptionCheckerMock) Check(
	ctx context.Context, owner PhaseObjectOwner, obj client.Object, previous []PreviousObjectSet,
) (needsAdoption bool, err error) {
	args := m.Called(ctx, owner, obj)
	return args.Bool(0), args.Error(1)
}

type patcherMock struct {
	mock.Mock
}

func (m *patcherMock) Patch(
	ctx context.Context,
	desiredObj, currentObj, updatedObj *unstructured.Unstructured,
) error {
	args := m.Called(ctx, desiredObj, currentObj, updatedObj)
	return args.Error(0)
}

type previousObjectSetMock struct {
	mock.Mock
}

func newPreviousObjectSetMockWithoutRemotes(
	obj client.Object,
) *previousObjectSetMock {
	m := &previousObjectSetMock{}
	m.On("ClientObject").Return(obj)
	m.On("GetRemotePhases").Return([]corev1alpha1.RemotePhaseReference{})
	return m
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
