//go:build mage
// +build mage

package main

import (
	"context"
	"os"

	"github.com/mt-sre/devkube/dev"
)

func Deploy(ctx context.Context) {
	if _, ok := os.LookupEnv("VERSION"); ok {
		panic("VERSION environment variable not set, please set an explicit version to deploy")
	}

	cluster, err := dev.NewCluster(locations.ClusterDeploymentCache(), dev.WithKubeconfigPath(os.Getenv("KUBECONFIG")))
	if err != nil {
		panic(err)
	}

	var d Dev
	d.deployPackageOperatorManager(ctx, cluster)
	d.deployPackageOperatorWebhook(ctx, cluster)
}
