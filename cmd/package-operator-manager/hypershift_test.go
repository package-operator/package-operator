package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clocktesting "k8s.io/utils/clock/testing"
)

type restMapperMock struct {
	mock.Mock
}

var (
	_       meta.RESTMapper = (*restMapperMock)(nil)
	errTest                 = errors.New("cheese happened")
)

func (m *restMapperMock) KindFor(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	args := m.Called()

	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *restMapperMock) KindsFor(schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	args := m.Called()

	return args.Get(0).([]schema.GroupVersionKind), args.Error(1)
}

func (m *restMapperMock) ResourceFor(schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	args := m.Called()

	return args.Get(0).(schema.GroupVersionResource), args.Error(1)
}

func (m *restMapperMock) ResourcesFor(schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	args := m.Called()

	return args.Get(0).([]schema.GroupVersionResource), args.Error(1)
}

func (m *restMapperMock) ResourceSingularizer(string) (string, error) {
	args := m.Called()

	return args.String(0), args.Error(1)
}

func (m *restMapperMock) RESTMappings(schema.GroupKind, ...string) ([]*meta.RESTMapping, error) {
	args := m.Called()

	return args.Get(0).([]*meta.RESTMapping), args.Error(1)
}

func (m *restMapperMock) RESTMapping(schema.GroupKind, ...string) (*meta.RESTMapping, error) {
	args := m.Called()

	return args.Get(0).(*meta.RESTMapping), args.Error(1)
}

func TestHypershift_needLeaderElection(t *testing.T) {
	t.Parallel()

	ticker := clocktesting.NewFakeClock(time.Time{}).NewTicker(hyperShiftPollInterval)
	h := newHypershift(testr.New(t), &restMapperMock{}, ticker)
	require.True(t, h.NeedLeaderElection())
}

func TestHypershift_start_foundImmediately(t *testing.T) {
	t.Parallel()

	clk := clocktesting.NewFakeClock(time.Time{})

	restMock := &restMapperMock{}
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, nil).Once()

	h := newHypershift(testr.New(t), restMock, clk.NewTicker(hyperShiftPollInterval))

	ctx := context.Background()

	clk.Step(hyperShiftPollInterval)
	require.ErrorIs(t, h.Start(ctx), ErrHypershiftAPIPostSetup)
}

func TestHypershift_start_foundOnSecondPoll(t *testing.T) {
	t.Parallel()

	clk := clocktesting.NewFakeClock(time.Time{})

	restMock := &restMapperMock{}
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, &meta.NoResourceMatchError{}).Run(func(args mock.Arguments) {
		clk.Step(hyperShiftPollInterval)
	}).Once()
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, nil).Run(func(args mock.Arguments) {
		clk.Step(hyperShiftPollInterval)
	}).Once()

	h := newHypershift(testr.New(t), restMock, clk.NewTicker(hyperShiftPollInterval))

	ctx := context.Background()

	clk.Step(hyperShiftPollInterval)
	require.ErrorIs(t, h.Start(ctx), ErrHypershiftAPIPostSetup)
}

func TestHypershift_start_someerr(t *testing.T) {
	t.Parallel()

	clk := clocktesting.NewFakeClock(time.Time{})

	restMock := &restMapperMock{}
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, errTest).Once()

	h := newHypershift(testr.New(t), restMock, clk.NewTicker(hyperShiftPollInterval))

	ctx := context.Background()

	clk.Step(hyperShiftPollInterval)
	require.ErrorIs(t, h.Start(ctx), errTest)
}

func TestHypershift_start_cancel(t *testing.T) {
	t.Parallel()

	clk := clocktesting.NewFakeClock(time.Time{})

	restMock := &restMapperMock{}
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, errTest).Once()

	h := newHypershift(testr.New(t), restMock, clk.NewTicker(hyperShiftPollInterval))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	err := h.Start(ctx)

	require.Error(t, ctx.Err())
	require.NoError(t, err)
}
