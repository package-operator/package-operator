package main

import (
	"context"
	"fmt"

	"pkg.package-operator.run/cardboard/run"
)

type Release struct{}

// TODO index out of bounds.
func (r Release) All(ctx context.Context, args []string) error {
	registry := "quay.io/package-operator"
	if len(args) > 2 {
		return fmt.Errorf("traget registry as a single arg or no args for official") //nolint:goerr113
	} else if len(args) == 1 {
		registry = args[1]
	}

	if registry == "" {
		return fmt.Errorf("registry may not be empty") //nolint:goerr113
	}

	return mgr.ParallelDeps(ctx, run.Meth1(r, r.All, args),
		run.Fn2(pushImage, "cli", registry),
		run.Fn2(pushImage, "package-operator-manager", registry),
		run.Fn2(pushImage, "package-operator-webhook", registry),
		run.Fn2(pushImage, "remote-phase-manager", registry),
		run.Fn2(pushImage, "test-stub", registry),
	)
}
