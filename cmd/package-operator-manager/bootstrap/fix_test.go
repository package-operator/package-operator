package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"package-operator.run/cmd/package-operator-manager/bootstrap/fix"
	"package-operator.run/internal/testutil"
)

func Test_fixer_happy_path(t *testing.T) {
	t.Parallel()

	// Two fixes, no errors, the first fix is not eligible for execution, the second is.
	fixNoRun := newRunCheckerMock().program(t, false, nil, nil)
	defer fixNoRun.AssertExpectations(t)
	fixRunNoErr := newRunCheckerMock().program(t, true, nil, nil)
	defer fixRunNoErr.AssertExpectations(t)

	fixer := newTestFixer(t, []runChecker{
		fixNoRun,
		fixRunNoErr,
	})

	err := fixer.fix(t.Context())
	require.NoError(t, err)
}

var errTest = errors.New("test")

func Test_fixer_stops_at_error(t *testing.T) {
	t.Parallel()

	// Three fixes, the first one succeeds, the second one errors, the third one should never be looked at.
	fixRunNoErr := newRunCheckerMock().program(t, true, nil, nil)
	defer fixRunNoErr.AssertExpectations(t)
	fixRunErr := newRunCheckerMock().program(t, true, nil, errTest)
	defer fixRunErr.AssertExpectations(t)
	fixShouldntCheckOrRun := newRunCheckerMock()
	defer fixShouldntCheckOrRun.AssertExpectations(t)

	fixer := newTestFixer(t, []runChecker{
		fixRunNoErr,
		fixRunErr,
		fixShouldntCheckOrRun,
	})

	err := fixer.fix(t.Context())
	require.ErrorIs(t, err, errTest)
}

func newTestFixer(t *testing.T, runCheckers []runChecker) *fixer {
	t.Helper()

	return &fixer{
		log:    testr.New(t),
		client: testutil.NewClient(),
		fixes:  runCheckers,
	}
}

// Force mock to implement the `runChecker` interface.
var _ runChecker = &runCheckerMock{}

type runCheckerMock struct {
	mock.Mock
}

func newRunCheckerMock() *runCheckerMock {
	return &runCheckerMock{}
}

func (s *runCheckerMock) program(t *testing.T, checkResult bool, checkErr error, runErr error) *runCheckerMock {
	t.Helper()

	isTypeFixCtx := mock.IsType(fix.Context{})

	// program `.Check(...)` with provided return values
	s.On("Check", mock.Anything, isTypeFixCtx).Return(checkResult, checkErr)

	// program `.Run(...)` if call to `.Check(...)` results in `true` and no error
	if checkResult && checkErr == nil {
		s.On("Run", mock.Anything, isTypeFixCtx).Return(runErr)
	}

	return s
}

func (s *runCheckerMock) Check(ctx context.Context, fc fix.Context) (bool, error) {
	args := s.Called(ctx, fc)
	return args.Bool(0), args.Error(1)
}

func (s *runCheckerMock) Run(ctx context.Context, fc fix.Context) error {
	args := s.Called(ctx, fc)
	return args.Error(0)
}
