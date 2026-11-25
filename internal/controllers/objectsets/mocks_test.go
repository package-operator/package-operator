package objectsets

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"pkg.package-operator.run/boxcutter/machinery"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/preflight"
)

type remotePhaseReconcilerMock struct {
	mock.Mock
}

func (m *remotePhaseReconcilerMock) Reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (machinery.PhaseResult, error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Get(0).(machinery.PhaseResult), args.Error(1)
}

func (m *remotePhaseReconcilerMock) Teardown(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Bool(0), args.Error(1)
}

type phasesCheckerMock struct {
	mock.Mock
}

func (pc *phasesCheckerMock) Check(
	ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase,
) (violations []preflight.Violation, err error) {
	args := pc.Called(ctx, phases)
	return args.Get(0).([]preflight.Violation), args.Error(1)
}

type clockMock struct {
	mock.Mock
}

func (m *clockMock) Now() time.Time {
	args := m.Called()

	return args.Get(0).(time.Time)
}
