package controllers

import (
	"context"

	"github.com/stretchr/testify/mock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/preflight"
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

func (m *phaseObjectOwnerMock) IsSpecPaused() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *phaseObjectOwnerMock) GetConditions() *[]metav1.Condition {
	args := m.Called()
	return args.Get(0).(*[]metav1.Condition)
}

type adoptionCheckerMock struct {
	mock.Mock
}

func (m *adoptionCheckerMock) Check(
	owner PhaseObjectOwner, obj client.Object, previous []PreviousObjectSet,
	collisionProtection corev1alpha1.CollisionProtection,
) (needsAdoption bool, err error) {
	args := m.Called(owner, obj, previous, collisionProtection)
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

type preflightCheckerMock struct {
	mock.Mock
}

func (m *preflightCheckerMock) Check(
	ctx context.Context, owner, obj client.Object,
) (violations []preflight.Violation, err error) {
	args := m.Called(ctx, owner, obj)
	return args.Get(0).([]preflight.Violation), args.Error(1)
}

type testUpdateMock struct {
	mock.Mock
}

func (m *testUpdateMock) Update(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type objectSetOrPhaseStub struct {
	ObjectSet corev1alpha1.ObjectSet
}

func (s *objectSetOrPhaseStub) ClientObject() client.Object {
	return &s.ObjectSet
}

func (s *objectSetOrPhaseStub) GetConditions() *[]metav1.Condition {
	return &s.ObjectSet.Status.Conditions
}
func (s *objectSetOrPhaseStub) UpdateStatusPhase() {}
