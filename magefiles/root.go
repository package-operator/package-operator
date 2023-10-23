//go:build mage
// +build mage

package main

import (
	"context"
	"os"

	"github.com/mt-sre/devkube/devcluster"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Deploy(ctx context.Context) {
	if _, ok := os.LookupEnv("VERSION"); ok {
		panic("VERSION environment variable not set, please set an explicit version to deploy")
	}

	kubeconfig, err := os.ReadFile(os.Getenv("KUBECONFIG"))
	cfg, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	must(err)
	restCfg, err := cfg.ClientConfig()
	must(err)
	cli, err := client.New(restCfg, client.Options{})
	must(err)

	cluster := devcluster.Cluster{Cli: cli}

	var d Dev
	d.deployPackageOperatorManager(ctx, cluster)
	d.deployPackageOperatorWebhook(ctx, cluster)
}
