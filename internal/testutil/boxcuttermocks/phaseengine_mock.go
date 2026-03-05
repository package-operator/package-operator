package boxcuttermocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/validation"

	"package-operator.run/internal/controllers/boxcutterutil"
)

type PhaseEngineFactoryMock struct {
	mock.Mock
}

func (f *PhaseEngineFactoryMock) New(accessor managedcache.Accessor) (boxcutterutil.PhaseEngine, error) {
	args := f.Called(accessor)
	return args.Get(0).(boxcutterutil.PhaseEngine), args.Error(1)
}

type PhaseEngineMock struct {
	mock.Mock
}

func (m *PhaseEngineMock) Reconcile(
	ctx context.Context,
	revision int64,
	phase types.Phase,
	opts ...types.PhaseReconcileOption,
) (machinery.PhaseResult, error) {
	args := m.Called(ctx, revision, phase, opts)
	return args.Get(0).(machinery.PhaseResult),
		args.Error(1)
}

func (m *PhaseEngineMock) Teardown(
	ctx context.Context,
	revision int64,
	phase types.Phase,
	opts ...types.PhaseTeardownOption,
) (machinery.PhaseTeardownResult, error) {
	args := m.Called(ctx, revision, phase, opts)
	return args.Get(0).(machinery.PhaseTeardownResult), args.Error(1)
}

type PhaseResultMock struct {
	mock.Mock
}

func (r *PhaseResultMock) GetName() string {
	args := r.Called()
	return args.Get(0).(string)
}

func (r *PhaseResultMock) GetValidationError() *validation.PhaseValidationError {
	args := r.Called()
	return args.Get(0).(*validation.PhaseValidationError)
}

func (r *PhaseResultMock) GetObjects() []machinery.ObjectResult {
	args := r.Called()
	return args.Get(0).([]machinery.ObjectResult)
}

func (r *PhaseResultMock) InTransition() bool {
	args := r.Called()
	return args.Get(0).(bool)
}

func (r *PhaseResultMock) IsComplete() bool {
	args := r.Called()
	return args.Bool(0)
}

func (r *PhaseResultMock) HasProgressed() bool {
	args := r.Called()
	return args.Get(0).(bool)
}

func (r *PhaseResultMock) GetProbesStatus() string {
	args := r.Called()
	return args.Get(0).(string)
}

func (r *PhaseResultMock) String() string {
	args := r.Called()
	return args.Get(0).(string)
}

type PhaseTeardownResultMock struct {
	mock.Mock
}

func (t *PhaseTeardownResultMock) GetName() string {
	args := t.Called()
	return args.Get(0).(string)
}

func (t *PhaseTeardownResultMock) IsComplete() bool {
	args := t.Called()
	return args.Bool(0)
}

func (t *PhaseTeardownResultMock) Gone() []types.ObjectRef {
	args := t.Called()
	return args.Get(0).([]types.ObjectRef)
}

func (t *PhaseTeardownResultMock) Waiting() []types.ObjectRef {
	args := t.Called()
	return args.Get(0).([]types.ObjectRef)
}

func (t *PhaseTeardownResultMock) String() string {
	args := t.Called()
	return args.Get(0).(string)
}
