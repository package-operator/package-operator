package objectsetphases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/probing"
	"package-operator.run/package-operator/internal/testutil"
)

type phaseReconcilerMock struct {
	mock.Mock
}

func (m *phaseReconcilerMock) ReconcilePhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
	probe probing.Prober, previous []controllers.PreviousObjectSet,
) error {
	m.Called(ctx, owner, phase, probe, previous)
	return nil
}

func (m *phaseReconcilerMock) TeardownPhase(
	ctx context.Context, owner controllers.PhaseObjectOwner,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	m.Called(ctx, owner, phase)
	return false, nil
}

func TestPhaseReconciler_Reconcile(t *testing.T) {

	scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
	prev := newGenericObjectSet(scheme)
	prevObj := prev.ClientObject()
	prevObj.SetName("test")

	prm := &phaseReconcilerMock{}

	prm.On("ReconcilePhase", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	r := newObjectSetPhaseReconciler(prm, func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{prev}, nil
	})

	osp := &GenericObjectSetPhase{
		ObjectSetPhase: corev1alpha1.ObjectSetPhase{
			Spec: corev1alpha1.ObjectSetPhaseSpec{
				Previous: []corev1alpha1.PreviousRevisionReference{
					{
						Name: "test",
					},
				},
				AvailabilityProbes: []corev1alpha1.ObjectSetProbe{
					{
						Probes: []corev1alpha1.Probe{
							{
								FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
									FieldA: ".metadata.name",
									FieldB: ".metadata.annotations.name",
								},
							},
						},
					},
				},
			},
		},
	}
	res, err := r.Reconcile(context.Background(), osp)
	assert.Empty(t, res)
	assert.NoError(t, err)
	prm.AssertCalled(t, "ReconcilePhase", mock.Anything, osp, osp.GetPhase(), osp.ObjectSetPhase.Spec.Previous, osp.ObjectSetPhase.Spec.AvailabilityProbes)
}
