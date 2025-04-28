package objectdeployments

import (
	"context"

	"github.com/stretchr/testify/mock"
	ctrl "sigs.k8s.io/controller-runtime"

	adapters "package-operator.run/internal/adapters"
)

var _ objectSetSubReconciler = (*objectSetSubReconcilerMock)(nil)

type objectSetSubReconcilerMock struct {
	mock.Mock
}

func (o *objectSetSubReconcilerMock) Reconcile(
	ctx context.Context, currentObjectSet adapters.ObjectSetAccessor,
	prevObjectSets []adapters.ObjectSetAccessor, objectDeployment adapters.ObjectDeploymentAccessor,
) (ctrl.Result, error) {
	args := o.Called(ctx, currentObjectSet, prevObjectSets, objectDeployment)
	err, _ := args.Get(1).(error)
	return args.Get(0).(ctrl.Result), err
}
