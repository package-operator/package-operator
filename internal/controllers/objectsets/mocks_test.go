package objectsets

import (
	"context"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/probing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/package-operator/internal/testutil"
)

type dynamicCacheMock struct {
	testutil.CtrlClient
}

func (c *dynamicCacheMock) Source() source.Source {
	args := c.Called()
	return args.Get(0).(source.Source)
}

func (c *dynamicCacheMock) Free(ctx context.Context, obj client.Object) error {
	args := c.Called(ctx, obj)
	return args.Error(0)
}

func (c *dynamicCacheMock) Watch(
	ctx context.Context, owner client.Object, obj runtime.Object,
) error {
	args := c.Called(ctx, owner, obj)
	return args.Error(0)
}

type remotePhaseReconcilerMock struct {
	mock.Mock
}

func (m *remotePhaseReconcilerMock) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (err error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Error(0)
}

func (m *remotePhaseReconcilerMock) Teardown(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Bool(0), args.Error(1)
}

type phaseReconcilerMock struct {
	mock.Mock
}

func (m *phaseReconcilerMock) ReconcilePhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []controllers.PreviousObjectSet,
) error {
	args := m.Called(ctx, owner, phase, probe, previous)
	return args.Error(0)
}

func (m *phaseReconcilerMock) TeardownPhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, owner, phase)
	return args.Bool(0), args.Error(1)
}
