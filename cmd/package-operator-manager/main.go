package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/dig"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"package-operator.run/package-operator/cmd/package-operator-manager/bootstrap"
	"package-operator.run/package-operator/cmd/package-operator-manager/components"
	hypershiftv1beta1 "package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/package-operator/internal/environment"
)

const (
	hyperShiftPollInterval = 10 * time.Second
)

var di *dig.Container

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

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
	if opts.PrintVersion {
		printVersion()
		return nil
	}

	if len(opts.CopyTo) > 0 {
		if err := runCopyTo(opts.CopyTo); err != nil {
			return fmt.Errorf("unable to run copy-to: %w", err)
		}
		return nil
	}

	ctx := ctrl.SetupSignalHandler()
	if len(opts.SelfBootstrap) > 0 {
		if err := di.Provide(bootstrap.NewBootstrapper); err != nil {
			return err
		}

		var (
			bs     *bootstrap.Bootstrapper
			envMgr *environment.Manager
		)
		if err := di.Invoke(func(
			lbs *bootstrap.Bootstrapper,
			mgr ctrl.Manager,
			bootstrapControllers components.BootstrapControllers,
			discoveryClient discovery.DiscoveryInterface,
		) error {
			bs = lbs
			envMgr = environment.NewManager(
				mgr.GetClient(), discoveryClient,
				append(environment.ImplementsSinker(
					bootstrapControllers.List(),
				), lbs))
			return mgr.Add(envMgr)
		}); err != nil {
			return err
		}

		if err := envMgr.Init(ctx); err != nil {
			return err
		}

		if err := bs.Bootstrap(ctx, func(ctx context.Context) error {
			// Lazy create manager after boot strapper is finished or
			// the RESTMapper will not pick up the CRDs in the cluster.
			return di.Invoke(func(
				mgr ctrl.Manager, bootstrapControllers components.BootstrapControllers,
				discoveryClient discovery.DiscoveryInterface,
			) error {
				if err := bootstrapControllers.SetupWithManager(mgr); err != nil {
					return err
				}

				return mgr.Start(ctx)
			})
		}); err != nil {
			return err
		}
		return nil
	}

	if err := di.Provide(newPackageOperatorManager); err != nil {
		return err
	}
	return di.Invoke(func(pkoMgr *packageOperatorManager) error {
		return pkoMgr.Start(ctx)
	})
}

func printVersion() {
	version := "binary compiled without version info"
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.String()
	}
	fmt.Fprintln(os.Stderr, version)
}

type packageOperatorManager struct {
	log logr.Logger
	mgr ctrl.Manager

	hostedClusterController components.HostedClusterController
	environmentManager      *environment.Manager
}

func newPackageOperatorManager(
	mgr ctrl.Manager, log logr.Logger,
	hostedClusterController components.HostedClusterController,
	discoveryClient discovery.DiscoveryInterface,
	allControllers components.AllControllers,
) (*packageOperatorManager, error) {
	if err := allControllers.SetupWithManager(mgr); err != nil {
		return nil, err
	}

	envMgr := environment.NewManager(
		mgr.GetClient(), discoveryClient,
		environment.ImplementsSinker(allControllers.List()))
	if err := mgr.Add(envMgr); err != nil {
		return nil, err
	}

	pkoMgr := &packageOperatorManager{
		log: log.WithName("package-operator-manager"),
		mgr: mgr,

		hostedClusterController: hostedClusterController,
		environmentManager:      envMgr,
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

	if err := pkoMgr.environmentManager.Init(ctx); err != nil {
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
	log := logr.FromContextOrDiscard(ctx)

	// Probe for HyperShift API
	_, err := pkoMgr.mgr.GetRESTMapper().
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

	case meta.IsNoMatchError(err):
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
