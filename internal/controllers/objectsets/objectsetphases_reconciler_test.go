package objectsets

//
//import (
//	"context"
//	"testing"
//
//	"package-operator.run/package-operator/internal/probing"
//
//	"github.com/stretchr/testify/mock"
//
//	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
//	"package-operator.run/package-operator/internal/controllers"
//)
//
////func newObjectSetPhasesReconciler(
////	phaseReconciler phaseReconciler,
////	remotePhase remotePhaseReconciler,
////	lookupPreviousRevisions lookupPreviousRevisions,
////) *objectSetPhasesReconciler {
////	return &objectSetPhasesReconciler{
////		phaseReconciler:         phaseReconciler,
////		remotePhase:             remotePhase,
////		lookupPreviousRevisions: lookupPreviousRevisions,
////	}
////}
//
//type remotePhaseReconcilerMock struct {
//	mock.Mock
//}
//
//func (m *remotePhaseReconcilerMock) Reconcile(
//	ctx context.Context, objectSet genericObjectSet,
//	phase corev1alpha1.ObjectSetTemplatePhase,
//) (err error) {
//	args := m.Called(ctx, objectSet, phase)
//	return args.Error(0)
//}
//
//func (m *remotePhaseReconcilerMock) Teardown(
//	ctx context.Context, objectSet genericObjectSet,
//	phase corev1alpha1.ObjectSetTemplatePhase,
//) (cleanupDone bool, err error) {
//	args := m.Called(ctx, objectSet, phase)
//	return args.Bool(0), args.Error(1)
//}
//
//type lookupPreviousRevisions func(
//	ctx context.Context, owner controllers.PreviousOwner,
//) ([]controllers.PreviousObjectSet, error)
//
//type phaseReconcilerMock struct {
//	mock.Mock
//}
//
//func (m *phaseReconcilerMock) ReconcilePhase(
//	ctx context.Context, owner controllers.PhaseObjectOwner,
//	phase corev1alpha1.ObjectSetTemplatePhase,
//	probe probing.Prober, previous []controllers.PreviousObjectSet,
//) error {
//	args := m.Called(ctx, owner, phase, probe, previous)
//	return args.Error(0)
//}
//
//func (m *phaseReconcilerMock) TeardownPhase(
//	ctx context.Context, owner controllers.PhaseObjectOwner,
//	phase corev1alpha1.ObjectSetTemplatePhase,
//) (cleanupDone bool, err error) {
//	args := m.Called(ctx, owner, phase)
//	return args.Bool(0), args.Error(1)
//}
//
//func TestPhaseReconciler_Reconcile(t *testing.T) {
//	pr := &phaseReconcilerMock{}
//	remotePr := &remotePhaseReconcilerMock{}
//	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
//		return []controllers.PreviousObjectSet{}, nil
//	}
//	newObjectSetPhasesReconciler(pr, remotePr, lookup)
//}
