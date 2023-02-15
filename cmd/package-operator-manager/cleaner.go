package main

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type cleaner struct {
	client client.Client
}

var (
	_ manager.Runnable               = (*cleaner)(nil)
	_ manager.LeaderElectionRunnable = (*cleaner)(nil)
)

func newCleaner(client client.Client) *cleaner { return &cleaner{client} }
func (h *cleaner) NeedLeaderElection() bool    { return true }

func (h *cleaner) Start(ctx context.Context) error {
	podList := corev1.PodList{}
	if err := h.client.List(ctx, &podList); err != nil {
		return fmt.Errorf("list pods for cleanup: %w", err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if shouldDeletePod(pod) {
			if err := h.client.Delete(ctx, pod); err != nil {
				return fmt.Errorf("cleanup old pod: %w", err)
			}
		}
	}

	return nil
}

func shouldDeletePod(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		switch {
		case condition.Type != corev1.PodReady:
		case condition.Reason == "PodCompleted" && condition.Status == corev1.ConditionFalse:
			return true
		case condition.Reason == "ContainersNotReady" && condition.Status == corev1.ConditionFalse:
			return true
		}
	}

	return false
}
