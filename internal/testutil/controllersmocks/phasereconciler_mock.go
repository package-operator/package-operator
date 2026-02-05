package controllersmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"pkg.package-operator.run/boxcutter/probing"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers"
)

type PhaseReconcilerMock struct {
	mock.Mock
}

func (m *PhaseReconcilerMock) ReconcilePhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []controllers.PreviousObjectSet,
) ([]client.Object, controllers.ProbingResult, error) {
	args := m.Called(ctx, owner, phase, probe, previous)
	return args.Get(0).([]client.Object),
		args.Get(1).(controllers.ProbingResult),
		args.Error(2)
}

func (m *PhaseReconcilerMock) TeardownPhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, owner, phase)
	return args.Bool(0), args.Error(1)
}
