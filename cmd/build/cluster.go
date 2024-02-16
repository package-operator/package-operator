package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"pkg.package-operator.run/cardboard/modules/kind"
	"pkg.package-operator.run/cardboard/modules/kubeclients"
	"pkg.package-operator.run/cardboard/run"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Cluster focused targets.
type Cluster struct {
	*kind.Cluster
	registryPort int32
}

// NewCluster prepares a configured cluster object.
func NewCluster(registryPort int32) Cluster {
	return Cluster{
		registryPort: registryPort,

		Cluster: kind.NewCluster("pko",
			kind.WithClusterConfig(kindv1alpha4.Cluster{
				ContainerdConfigPatches: []string{
					// Replace `imageRegistry` with our local dev-registry.
					fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s"]
endpoint = ["http://localhost:31320"]`, imageRegistryHost()),
				},
				Nodes: []kindv1alpha4.Node{
					{
						Role: kindv1alpha4.ControlPlaneRole,
						ExtraPortMappings: []kindv1alpha4.PortMapping{
							// Open port to enable connectivity with local registry.
							{
								ContainerPort: 5001,
								HostPort:      registryPort,
								ListenAddress: "127.0.0.1",
								Protocol:      "TCP",
							},
						},
					},
				},
			}),
			kind.WithClientOptions{
				kubeclients.WithSchemeBuilder{corev1alpha1.AddToScheme},
			},
			kind.WithClusterInitializers{
				kind.ClusterLoadObjectsFromFiles{filepath.Join("config", "local-registry.yaml")},
			},
		),
	}
}

// Creates the local development cluster.
func (c *Cluster) create(ctx context.Context) error {
	self := run.Meth(c, c.create)
	return mgr.SerialDeps(ctx, self,
		c, // cardboard's internal cluster magic
		run.Meth1(c, c.loadImages, c.registryPort),
	)
}

// Destroys the local development cluster.
func (c *Cluster) destroy(ctx context.Context) error {
	return c.Destroy(ctx)
}

// Load images into the local development cluster.
func (c *Cluster) loadImages(ctx context.Context, registryPort int32) error {
	self := run.Meth1(c, c.loadImages, registryPort)

	hostPort := fmt.Sprintf("localhost:%d", registryPort)
	registry := localRegistry(hostPort)

	if err := mgr.ParallelDeps(ctx, self,
		run.Fn2(pushImage, "package-operator-manager", registry),
		run.Fn2(pushImage, "package-operator-webhook", registry),
		run.Fn2(pushImage, "remote-phase-manager", registry),
		run.Fn2(pushImage, "test-stub", registry),
	); err != nil {
		return err
	}

	if err := os.Setenv("PKO_REPOSITORY_HOST", hostPort); err != nil {
		return err
	}

	if err := mgr.ParallelDeps(ctx, self,
		run.Fn2(pushPackage, "remote-phase", registry),
		run.Fn2(pushPackage, "test-stub", registry),
		run.Fn2(pushPackage, "test-stub-multi", registry),
	); err != nil {
		return err
	}

	// This needs to be separate because the remote-phase package image has to be pushed before
	// downstream dependencies of the package-operator package image can be regenerated.
	// *very very sad @erdii noises*
	if err := mgr.ParallelDeps(ctx, self,
		run.Fn2(pushPackage, "package-operator", registry),
	); err != nil {
		return err
	}

	return os.Unsetenv("PKO_REPOSITORY_HOST")
}
