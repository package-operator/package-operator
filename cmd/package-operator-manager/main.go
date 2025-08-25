package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/dig"
	"go.uber.org/zap/zapcore"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/utils/clock"
	"pkg.package-operator.run/boxcutter/managedcache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"package-operator.run/cmd/package-operator-manager/bootstrap"
	"package-operator.run/cmd/package-operator-manager/components"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/version"
)

const (
	hyperShiftPollInterval = 10 * time.Second
)

var di *dig.Container

func main() {
	var err error
	di, err = components.NewComponents()
	if err != nil {
		panic(err)
	}

	if err := di.Invoke(run); err != nil {
		panic(err)
	}
}

func run(opts components.Options) error {
	zapLevel := zapcore.Level(-1 * opts.LogLevel)
	zapOpts := zap.Options{
		Development: false,
		Level:       zapLevel,
	}
	logger := zap.New(zap.UseFlagOptions(&zapOpts))
	ctrl.SetLogger(logger)
	if opts.PrintVersion != nil {
		_ = version.Get().Write(opts.PrintVersion)

		return nil
	}

	// Use the explicitly created logger instance to ensure we have a valid logger in context
	ctx := logr.NewContext(ctrl.SetupSignalHandler(), logger)
	if len(opts.SelfBootstrap) > 0 {
		if err := di.Provide(bootstrap.NewBootstrapper); err != nil {
			return err
		}

		var (
			bs     *bootstrap.Bootstrapper
			envMgr *environment.Manager
		)
		if err := di.Invoke(func(lbs *bootstrap.Bootstrapper, _ components.UncachedClient, lenvMgr *environment.Manager) {
			bs = lbs
			envMgr = lenvMgr
		}); err != nil {
			return err
		}

		if err := envMgr.Init(ctx, []environment.Sinker{bs}); err != nil {
			return err
		}

		if err := bs.Bootstrap(ctx, func(bootstrapCtx context.Context) error {
			// the RESTMapper will not pick up the new CRDs in the cluster.
			return di.Invoke(func(
				mgr ctrl.Manager,
				bootstrapControllers components.BootstrapControllers,
				accessManager managedcache.ObjectBoundAccessManager[client.Object],
				_ discovery.DiscoveryInterface,
			) error {
				// TODO: refactor?
				if err := mgr.Add(accessManager); err != nil {
					return err
				}
				if err := bootstrapControllers.SetupWithManager(mgr); err != nil {
					return err
				}
				if err := envMgr.Init(bootstrapCtx, environment.ImplementsSinker(bootstrapControllers.List())); err != nil {
					return err
				}
				if err := mgr.Add(envMgr); err != nil {
					return err
				}

				return mgr.Start(bootstrapCtx)
			})
		}); err != nil {
			return err
		}
		return nil
	}

	if err := di.Provide(newPackageOperatorManager); err != nil {
		return err
	}
	return di.Invoke(func(
		pkoMgr *packageOperatorManager,
		accessManager managedcache.ObjectBoundAccessManager[client.Object],
	) error {
		// TODO: refactor?
		if err := pkoMgr.mgr.Add(accessManager); err != nil {
			return err
		}

		return pkoMgr.Start(ctx)
	})
}

type packageOperatorManager struct {
	log logr.Logger
	mgr ctrl.Manager

	hostedClusterController components.HostedClusterController
	environmentManager      *environment.Manager
	allControllers          components.AllControllers
}

func newPackageOperatorManager(
	mgr ctrl.Manager, log logr.Logger,
	hostedClusterController components.HostedClusterController,
	envMgr *environment.Manager,
	allControllers components.AllControllers,
) (*packageOperatorManager, error) {
	if err := allControllers.SetupWithManager(mgr); err != nil {
		return nil, err
	}
	if err := mgr.Add(envMgr); err != nil {
		return nil, err
	}

	pkoMgr := &packageOperatorManager{
		log: log.WithName("package-operator-manager"),
		mgr: mgr,

		hostedClusterController: hostedClusterController,
		environmentManager:      envMgr,
		allControllers:          allControllers,
	}

	return pkoMgr, nil
}

func (pkoMgr *packageOperatorManager) Start(ctx context.Context) error {
	log := pkoMgr.log
	ctx = logr.NewContext(ctx, log)
	log.Info("starting manager")

	if err := pkoMgr.probeHyperShiftIntegration(ctx); err != nil {
		return fmt.Errorf("setting up HyperShift integration: %w", err)
	}

	if err := pkoMgr.environmentManager.Init(
		ctx, environment.ImplementsSinker(pkoMgr.allControllers.List()),
	); err != nil {
		return fmt.Errorf("environment init: %w", err)
	}

	err := pkoMgr.mgr.Start(ctx)
	switch {
	case err == nil || errors.Is(err, ErrHypershiftAPIPostSetup):
		return nil
	default:
		return fmt.Errorf("problem running manager: %w", err)
	}
}

var hostedClusterGVK = hypershiftv1beta1.GroupVersion.
	WithKind("HostedCluster")

func (pkoMgr *packageOperatorManager) probeHyperShiftIntegration(
	ctx context.Context,
) error {
	log, err := logr.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("getting logger from context in probeHyperShiftIntegration: %w", err)
	}

	// Probe for HyperShift API
	_, err = pkoMgr.mgr.GetRESTMapper().
		RESTMapping(hostedClusterGVK.GroupKind(), hostedClusterGVK.Version)

	switch {
	case err == nil:
		// HyperShift HostedCluster API is present on the cluster
		// Auto-Enable HyperShift integration controller:
		log.Info("detected HostedCluster API, enabling HyperShift integration")
		if err = pkoMgr.hostedClusterController.
			SetupWithManager(pkoMgr.mgr); err != nil {
			return fmt.Errorf(
				"unable to create controller for HostedCluster: %w", err)
		}

	case meta.IsNoMatchError(err) || apimachineryerrors.IsNotFound(err) ||
		discovery.IsGroupDiscoveryFailedError(errors.Unwrap(err)):
		ticker := clock.RealClock{}.NewTicker(hyperShiftPollInterval)
		if err := pkoMgr.mgr.Add(
			newHypershift(
				log.WithName("HyperShift"),
				pkoMgr.mgr.GetRESTMapper(),
				ticker,
			),
		); err != nil {
			return fmt.Errorf("add hypershift checker: %w", err)
		}

	default:
		return fmt.Errorf("hypershiftv1beta1 probing: %w", err)
	}

	return nil
}
