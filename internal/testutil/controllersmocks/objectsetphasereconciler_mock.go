package controllersmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/adapters"
)

type ObjectSetPhasesReconcilerMock struct {
	mock.Mock
}

func (r *ObjectSetPhasesReconcilerMock) Reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (res ctrl.Result, err error) {
	args := r.Called(ctx, objectSet)
	return args.Get(0).(ctrl.Result), args.Error(1)
}

func (r *ObjectSetPhasesReconcilerMock) Teardown(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (cleanupDone bool, err error) {
	args := r.Called(ctx, objectSet)
	return args.Bool(0), args.Error(1)
}

type RevisionReconcilerMock struct {
	mock.Mock
}

func (r *RevisionReconcilerMock) Reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (res ctrl.Result, err error) {
	args := r.Called(ctx, objectSet)
	return args.Get(0).(ctrl.Result), args.Error(1)
}
