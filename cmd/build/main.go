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
	shr = sh.New()
	mgr = run.New(run.WithSources(source))

	appVersion = mustVersion()

	// internal modules.
	generate Generate
	test     Test
	lint     Lint
	cluster  = NewCluster("pko",
		withLocalRegistry(imageRegistryHost(), devClusterRegistryPort),
	)
	hypershiftHostedCluster = NewCluster("pko-hs-hc",
		withRegistryHostOverrideToOtherCluster(imageRegistryHost(), cluster),
	)

	//go:embed *.go
	source embed.FS
)

func main() {
	ctx := context.Background()

	err := errors.Join(
		// Required by cardboard itself.
		mgr.RegisterGoTool("crane", "github.com/google/go-containerregistry/cmd/crane", "0.19.1"),
		// Our deps
		mgr.RegisterGoTool("gotestfmt", "github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt", "2.5.0"),
		mgr.RegisterGoTool("controller-gen", "sigs.k8s.io/controller-tools/cmd/controller-gen", "0.15.0"),
		mgr.RegisterGoTool("conversion-gen", "k8s.io/code-generator/cmd/conversion-gen", "0.29.4"),
		mgr.RegisterGoTool("golangci-lint", "github.com/golangci/golangci-lint/cmd/golangci-lint", "1.58.1"),
		mgr.RegisterGoTool("k8s-docgen", "github.com/thetechnick/k8s-docgen", "0.6.2"),
		mgr.RegisterGoTool("helm", "helm.sh/helm/v3/cmd/helm", "3.14.4"),
		mgr.Register(&Dev{}, &CI{}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s\n", err)
		os.Exit(1)
	}

	if err := mgr.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s\n", err)
		os.Exit(1)
	}
}
