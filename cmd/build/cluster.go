package main

import (
	"context"
	"os"

	"pkg.package-operator.run/cardboard/modules/kind"
	"pkg.package-operator.run/cardboard/run"
)

// Cluster focused targets.
type Cluster struct {
	*kind.Cluster
}

// Creates the local development cluster.
func (c *Cluster) create(ctx context.Context) error {
	self := run.Meth(c, c.create)

	if err := mgr.SerialDeps(ctx, self, c); err != nil {
		return err
	}

	if err := os.MkdirAll(".cache/integration", 0o755); err != nil {
		return err
	}

	err := mgr.ParallelDeps(ctx, self,
		run.Fn2(pushImage, "package-operator-manager", "localhost:5001/package-operator"),
		run.Fn2(pushImage, "package-operator-webhook", "localhost:5001/package-operator"),
		run.Fn2(pushImage, "remote-phase-manager", "localhost:5001/package-operator"),
		run.Fn2(pushImage, "test-stub", "localhost:5001/package-operator"),
	)
	if err != nil {
		return err
	}

	if err := os.Setenv("PKO_REPOSITORY_HOST", "localhost:5001"); err != nil {
		return err
	}

	err = mgr.ParallelDeps(ctx, self,
		run.Fn2(pushPackage, "remote-phase", "localhost:5001/package-operator"),
		run.Fn2(pushPackage, "test-stub", "localhost:5001/package-operator"),
		run.Fn2(pushPackage, "test-stub-multi", "localhost:5001/package-operator"),
	)
	if err != nil {
		return err
	}

	err = mgr.ParallelDeps(ctx, self,
		run.Fn2(pushPackage, "package-operator", "localhost:5001/package-operator"),
	)
	if err != nil {
		return err
	}

	if err := os.Unsetenv("PKO_REPOSITORY_HOST"); err != nil {
		return err
	}

	return nil
}

// Destroys the local development cluster.
func (c *Cluster) destroy(ctx context.Context) error {
	return c.Destroy(ctx)
}
