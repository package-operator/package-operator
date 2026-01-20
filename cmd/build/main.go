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
	chart    Chart
	compile  Compile

	cluster = NewCluster("pko",
		withLocalRegistry(imageRegistryHost(), devClusterRegistryPort, devClusterRegistryAuthPort),
		withNodeLabels(map[string]string{"hypershift-affinity-test-label": "true"}),
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
		// TODO: unable to update, required go1.24
		mgr.RegisterGoTool("crane", "github.com/google/go-containerregistry/cmd/crane", "0.20.7"),
		// Our deps
		mgr.RegisterGoTool("gotestfmt", "github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt", "2.5.0"),
		// TODO: unable to update, required go1.24
		mgr.RegisterGoTool("controller-gen", "sigs.k8s.io/controller-tools/cmd/controller-gen", "0.20.0"),
		// TODO: unable to update, required go1.24
		mgr.RegisterGoTool("conversion-gen", "k8s.io/code-generator/cmd/conversion-gen", "0.35.0"),
		mgr.RegisterGoTool("golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", "2.8.0"),
		mgr.RegisterGoTool("k8s-docgen", "github.com/thetechnick/k8s-docgen", "0.6.4"),
		// TODO: unable to update, required go1.24
		mgr.RegisterGoTool("helm", "helm.sh/helm/v3/cmd/helm", "3.18.5"),
		mgr.RegisterGoTool("govulncheck", "golang.org/x/vuln/cmd/govulncheck", "1.1.4"),
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
