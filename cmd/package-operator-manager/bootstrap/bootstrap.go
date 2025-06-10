package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/cmd/package-operator-manager/bootstrap/proxy"
	"package-operator.run/cmd/package-operator-manager/components"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/packages"
)

const packageOperatorDeploymentName = "package-operator-manager"

type Bootstrapper struct {
	*environment.Sink

	client client.Client
	log    logr.Logger
	init   func(ctx context.Context) (
		needsBootstrap bool, err error,
	)
	fix func(ctx context.Context) error

	pkoNamespace string
}

func NewBootstrapper(
	scheme *runtime.Scheme, log logr.Logger,
	uncachedClient components.UncachedClient,
	registry *packages.RequestManager,
	opts components.Options,
) (*Bootstrapper, error) {
	c := uncachedClient
	init := newInitializer(
		c, scheme, &packageObjectLoad{},
		registry.Pull, opts.Namespace, opts.SelfBootstrap, opts.SelfBootstrapConfig, opts.ImagePrefixOverrides,
	)
	fixer := newFixer(c, log, opts.Namespace)

	return &Bootstrapper{
		Sink: environment.NewSink(uncachedClient),

		log:    log.WithName("bootstrapper"),
		client: c,
		init:   init.Init,
		fix:    fixer.fix,

		pkoNamespace: opts.Namespace,
	}, nil
}

func (b *Bootstrapper) Bootstrap(ctx context.Context, runManager func(ctx context.Context) error) error {
	ctx = logr.NewContext(ctx, b.log)
	log := b.log

	env, err := b.GetEnvironment(ctx, "")
	if err != nil {
		return fmt.Errorf("get environment: %w", err)
	}
	if err := proxy.RestartPKOWithEnvvarsIfNeeded(log, env); err != nil {
		return err
	}

	log.Info("running self-bootstrap")
	defer log.Info("self-bootstrap done")

	needsBootstrap, err := b.init(ctx)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}

	if err := b.fix(ctx); err != nil {
		return fmt.Errorf("fix: %w", err)
	}

	if needsBootstrap {
		if err := b.bootstrap(ctx, runManager); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
	}

	return nil
}

func (b *Bootstrapper) bootstrap(ctx context.Context, runManager func(ctx context.Context) error) error {
	// Stop manager when Package Operator is installed.
	ctx, cancel := context.WithCancel(ctx)
	go b.cancelWhenPackageAvailable(ctx, cancel)

	// TODO(jgwosdz): investigate if it would make sense to stop using envvars and instead go
	// through a central configuration facility (like opts?)

	// Force Adoption of objects during initial bootstrap to take ownership of
	// CRDs, Namespace, ServiceAccount and ClusterRoleBinding.
	if err := os.Setenv(constants.ForceAdoptionEnvironmentVariable, "1"); err != nil {
		return err
	}
	if err := runManager(ctx); err != nil {
		return fmt.Errorf("running manager for self-bootstrap: %w", err)
	}
	return nil
}

func (b *Bootstrapper) cancelWhenPackageAvailable(
	ctx context.Context, cancel context.CancelFunc,
) {
	// Get logger from context and properly handle error
	log, err := logr.FromContext(ctx)
	if err != nil {
		panic(fmt.Errorf("getting logger from context in cancelWhenPackageAvailable: %w", err))
	}
	err = wait.PollUntilContextCancel(
		ctx, packageOperatorPackageCheckInterval, true,
		func(ctx context.Context) (done bool, err error) {
			available, err := isPKOAvailable(ctx, b.client, b.pkoNamespace)
			if err != nil {
				return false, err
			}
			log.Info("waiting for PKO to become available", "available", available)
			return available, nil
		})
	if err != nil {
		panic(err)
	}

	log.Info("Package Operator bootstrapped successfully!")
	cancel()
}
