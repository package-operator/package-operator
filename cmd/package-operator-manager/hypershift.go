package main

import (
	"context"
	"errors"
	"fmt"

	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

type hypershift struct {
	log    logr.Logger
	mapper meta.RESTMapper
	ticker clock.Ticker
}

var (
	_ manager.Runnable               = (*hypershift)(nil)
	_ manager.LeaderElectionRunnable = (*hypershift)(nil)

	ErrHypershiftAPIPostSetup = errors.New("detected hypershift installation after setup completed")
)

func newHypershift(log logr.Logger, mapper meta.RESTMapper, ticker clock.Ticker) *hypershift {
	return &hypershift{log, mapper, ticker}
}

func (h *hypershift) NeedLeaderElection() bool { return true }

func (h *hypershift) Start(ctx context.Context) error {
	defer h.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-h.ticker.C():
		}

		// Probe for HyperShift API
		hostedClusterGVK := hypershiftv1beta1.GroupVersion.WithKind("HostedCluster")
		_, err := h.mapper.RESTMapping(hostedClusterGVK.GroupKind(), hostedClusterGVK.Version)
		switch {
		case err == nil:
			h.log.Info("detected hypershift installation after setup completed, restarting operator")
			return ErrHypershiftAPIPostSetup
		case meta.IsNoMatchError(err) || apimachineryerrors.IsNotFound(err) || discovery.IsGroupDiscoveryFailedError(errors.Unwrap(err)):
			continue
		default:
			return fmt.Errorf("hypershiftv1beta1 probing: %w", err)
		}
	}
}
