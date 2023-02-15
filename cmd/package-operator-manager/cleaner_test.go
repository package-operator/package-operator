package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"package-operator.run/package-operator/internal/testutil"
)

func TestCleanup_needLeaderElection(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()
	c := newCleaner(client)
	require.True(t, c.NeedLeaderElection())
}

func TestCleanup_start_nopods(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()
	c := newCleaner(client)
	ctx := context.Background()

	client.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	require.NoError(t, c.Start(ctx))
}

func TestCleanup_start_listerr(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()
	c := newCleaner(client)
	ctx := context.Background()

	client.On("List", mock.Anything, mock.Anything, mock.Anything).Return(errTest)

	require.ErrorIs(t, c.Start(ctx), errTest)
}

func TestCleanup_start_podnoconditions(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()
	c := newCleaner(client)
	ctx := context.Background()

	client.On("List", mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*corev1.PodList)
			list.Items = []corev1.Pod{{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			}}
		},
	).Return(nil)

	require.NoError(t, c.Start(ctx))
}

func TestCleanup_start_podcomplete(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()
	c := newCleaner(client)
	ctx := context.Background()

	client.On("List", mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*corev1.PodList)
			list.Items = []corev1.Pod{{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Reason: "PodCompleted",
						Status: corev1.ConditionFalse,
					}},
				},
			}}
		},
	).Return(nil)
	client.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	require.NoError(t, c.Start(ctx))
}

func TestCleanup_start_podnotready(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()
	c := newCleaner(client)
	ctx := context.Background()

	client.On("List", mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*corev1.PodList)
			list.Items = []corev1.Pod{{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Reason: "ContainersNotReady",
						Status: corev1.ConditionFalse,
					}},
				},
			}}
		},
	).Return(nil)
	client.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	require.NoError(t, c.Start(ctx))
}

func TestCleanup_start_deleteerr(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()
	c := newCleaner(client)
	ctx := context.Background()

	client.On("List", mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*corev1.PodList)
			list.Items = []corev1.Pod{{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Reason: "ContainersNotReady",
						Status: corev1.ConditionFalse,
					}},
				},
			}}
		},
	).Return(nil)
	client.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(errTest).Once()

	require.ErrorIs(t, c.Start(ctx), errTest)
}
