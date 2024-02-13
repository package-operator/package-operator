package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"

	"pkg.package-operator.run/cardboard/run"
	"pkg.package-operator.run/cardboard/sh"
)

var (
	shr        *sh.Runner
	mgr        *run.Manager
	appVersion string

	// internal modules.
	generate *Generate
	test     *Test
	lint     *Lint
	cluster  *Cluster

	//go:embed *.go
	source embed.FS
)

func main() {
	ctx := context.Background()

	mgr = run.New(run.WithSources(source))
	shr = sh.New()
	generate = &Generate{}
	test = &Test{}
	lint = &Lint{}

	cluster = NewCluster()

	appVersion = mustVersion()

	err := errors.Join(
		// Required by cardboard itself.
		mgr.RegisterGoTool("crane", "github.com/google/go-containerregistry/cmd/crane", "0.19.0"),
		mgr.RegisterGoTool("kind", "sigs.k8s.io/kind", "0.21.0"),
		// Our deps
		mgr.RegisterGoTool("gotestfmt", "github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt", "2.5.0"),
		mgr.RegisterGoTool("controller-gen", "sigs.k8s.io/controller-tools/cmd/controller-gen", "0.14.0"),
		mgr.RegisterGoTool("conversion-gen", "k8s.io/code-generator/cmd/conversion-gen", "0.29.1"),
		mgr.RegisterGoTool("golangci-lint", "github.com/golangci/golangci-lint/cmd/golangci-lint", "1.55.2"),
		mgr.RegisterGoTool("k8s-docgen", "github.com/thetechnick/k8s-docgen", "0.6.2"),
		mgr.RegisterGoTool("helm", "helm.sh/helm/v3/cmd/helm", "3.14.0"),
		mgr.Register(&Dev{}, &CI{}),
	)
	if err != nil {
		panic(err)
	}

	if err := mgr.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s\n", err)
		os.Exit(1)
	}
}
