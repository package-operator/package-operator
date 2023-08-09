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
	clocktesting "k8s.io/utils/clock/testing"

	"package-operator.run/internal/testutil/restmappermock"
)

var errTest = errors.New("cheese happened")

func TestHypershift_needLeaderElection(t *testing.T) {
	t.Parallel()

	ticker := clocktesting.NewFakeClock(time.Time{}).NewTicker(hyperShiftPollInterval)
	h := newHypershift(testr.New(t), &restmappermock.RestMapperMock{}, ticker)
	require.True(t, h.NeedLeaderElection())
}

func TestHypershift_start_foundImmediately(t *testing.T) {
	t.Parallel()

	clk := clocktesting.NewFakeClock(time.Time{})

	restMock := &restmappermock.RestMapperMock{}
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, nil).Once()

	h := newHypershift(testr.New(t), restMock, clk.NewTicker(hyperShiftPollInterval))

	ctx := context.Background()

	clk.Step(hyperShiftPollInterval)
	require.ErrorIs(t, h.Start(ctx), ErrHypershiftAPIPostSetup)
}

func TestHypershift_start_foundOnSecondPoll(t *testing.T) {
	t.Parallel()

	clk := clocktesting.NewFakeClock(time.Time{})

	restMock := &restmappermock.RestMapperMock{}
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

	restMock := &restmappermock.RestMapperMock{}
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, errTest).Once()

	h := newHypershift(testr.New(t), restMock, clk.NewTicker(hyperShiftPollInterval))

	ctx := context.Background()

	clk.Step(hyperShiftPollInterval)
	require.ErrorIs(t, h.Start(ctx), errTest)
}

func TestHypershift_start_cancel(t *testing.T) {
	t.Parallel()

	clk := clocktesting.NewFakeClock(time.Time{})

	restMock := &restmappermock.RestMapperMock{}
	restMock.On("RESTMapping").Return(&meta.RESTMapping{}, errTest).Once()

	h := newHypershift(testr.New(t), restMock, clk.NewTicker(hyperShiftPollInterval))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	err := h.Start(ctx)

	require.Error(t, ctx.Err())
	require.NoError(t, err)
}
