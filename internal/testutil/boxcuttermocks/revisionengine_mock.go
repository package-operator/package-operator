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

var (
	_ boxcutterutil.RevisionEngine        = (*RevisionEngineMock)(nil)
	_ boxcutterutil.RevisionEngineFactory = (*RevisionEngineFactoryMock)(nil)
	_ machinery.RevisionResult            = (*RevisionResultMock)(nil)
	_ machinery.RevisionTeardownResult    = (*RevisionTeardownResultMock)(nil)
)

type RevisionEngineFactoryMock struct {
	mock.Mock
}

func (f *RevisionEngineFactoryMock) New(accessor managedcache.Accessor) (boxcutterutil.RevisionEngine, error) {
	args := f.Called(accessor)
	return args.Get(0).(boxcutterutil.RevisionEngine), args.Error(1)
}

type RevisionEngineMock struct {
	mock.Mock
}

func (m *RevisionEngineMock) Reconcile(
	ctx context.Context,
	revision types.Revision,
	opts ...types.RevisionReconcileOption,
) (machinery.RevisionResult, error) {
	args := m.Called(ctx, revision, opts)
	return args.Get(0).(machinery.RevisionResult),
		args.Error(1)
}

func (m *RevisionEngineMock) Teardown(
	ctx context.Context,
	revision types.Revision,
	opts ...types.RevisionTeardownOption,
) (machinery.RevisionTeardownResult, error) {
	args := m.Called(ctx, revision, opts)
	return args.Get(0).(machinery.RevisionTeardownResult), args.Error(1)
}

type RevisionResultMock struct {
	mock.Mock
}

func (r *RevisionResultMock) GetPhases() []machinery.PhaseResult {
	args := r.Called()
	return args.Get(0).([]machinery.PhaseResult)
}

func (r *RevisionResultMock) GetValidationError() *validation.RevisionValidationError {
	args := r.Called()
	return args.Get(0).(*validation.RevisionValidationError)
}

func (r *RevisionResultMock) InTransition() bool {
	args := r.Called()
	return args.Bool(0)
}

func (r *RevisionResultMock) IsComplete() bool {
	args := r.Called()
	return args.Bool(0)
}

func (r *RevisionResultMock) HasProgressed() bool {
	args := r.Called()
	return args.Bool(0)
}

type RevisionTeardownResultMock struct {
	mock.Mock
}

func (t *RevisionTeardownResultMock) IsComplete() bool {
	args := t.Called()
	return args.Bool(0)
}

func (t *RevisionTeardownResultMock) GetPhases() []machinery.PhaseTeardownResult {
	args := t.Called()
	return args.Get(0).([]machinery.PhaseTeardownResult)
}

func (t *RevisionTeardownResultMock) GetWaitingPhaseNames() []string {
	args := t.Called()
	return args.Get(0).([]string)
}

func (t *RevisionTeardownResultMock) GetActivePhaseName() (string, bool) {
	args := t.Called()
	return args.Get(0).(string), args.Bool(1)
}

func (t *RevisionTeardownResultMock) GetGonePhaseNames() []string {
	args := t.Called()
	return args.Get(0).([]string)
}
