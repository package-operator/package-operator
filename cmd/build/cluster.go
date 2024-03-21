package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"pkg.package-operator.run/cardboard/modules/kind"
	"pkg.package-operator.run/cardboard/modules/kubeclients"
	"pkg.package-operator.run/cardboard/run"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	hsv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

// Cluster focused targets.
type Cluster struct {
	*kind.Cluster
	registryHostPort int32
}

type clusterConfigLocalRegistry struct {
	hostOverride string
	hostPort     int32
}

type clusterConfig struct {
	name                  string
	registryHostOverrides []struct {
		host, endpoint string
	}
	localRegistry *clusterConfigLocalRegistry
}

func (cc *clusterConfig) apply(opts ...clusterOption) {
	for _, opt := range opts {
		opt(cc)
	}
}

type clusterOption func(*clusterConfig)

func withRegistryHostOverride(host, endpoint string) clusterOption {
	return func(cc *clusterConfig) {
		cc.registryHostOverrides = append(cc.registryHostOverrides, struct {
			host     string
			endpoint string
		}{
			host:     host,
			endpoint: endpoint,
		})
	}
}

func withRegistryHostOverrideToOtherCluster(host string, targetCluster Cluster) clusterOption {
	return withRegistryHostOverride(host, targetCluster.Name()+"-control-plane")
}

func withLocalRegistry(hostOverride string, hostPort int32) clusterOption {
	return func(cc *clusterConfig) {
		cc.localRegistry = &clusterConfigLocalRegistry{
			hostPort:     hostPort,
			hostOverride: hostOverride,
		}
	}
}

func registryOverrideToml(override registryHostOverride) string {
	return fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s"]
endpoint = ["%s"]`, override.host, override.endpoint)
}

type registryHostOverride struct {
	host     string
	endpoint string
}

// NewCluster prepares a configured cluster object.
func NewCluster(name string, opts ...clusterOption) Cluster {
	cfg := &clusterConfig{
		name: name,
	}
	cfg.apply(opts...)

	var clusterInitializers []kind.ClusterInitializer
	cluster := Cluster{}

	containerdConfigPatches := []string{}
	for _, registryHostOverride := range cfg.registryHostOverrides {
		containerdConfigPatches = append(containerdConfigPatches, registryOverrideToml(registryHostOverride))
	}

	var extraPortMappings []kindv1alpha4.PortMapping

	if cfg.localRegistry != nil {
		cluster.registryHostPort = cfg.localRegistry.hostPort // todo rename field on cluster
		clusterInitializers = append(clusterInitializers,
			kind.ClusterLoadObjectsFromFiles{filepath.Join("config", "local-registry.yaml")})
		containerdConfigPatches = append(containerdConfigPatches, registryOverrideToml(registryHostOverride{
			host:     cfg.localRegistry.hostOverride,
			endpoint: "http://localhost:31320",
		}))
		extraPortMappings = append(extraPortMappings, kindv1alpha4.PortMapping{
			ContainerPort: 5001,
			HostPort:      cfg.localRegistry.hostPort,
			ListenAddress: "127.0.0.1",
			Protocol:      "TCP",
		})
	}

	cluster.Cluster = kind.NewCluster(cfg.name,
		kind.WithClusterConfig{
			ContainerdConfigPatches: containerdConfigPatches,
			Nodes: []kindv1alpha4.Node{
				{
					Role:              kindv1alpha4.ControlPlaneRole,
					ExtraPortMappings: extraPortMappings,
				},
			},
		},
		kind.WithClientOptions{
			kubeclients.WithSchemeBuilder{corev1alpha1.AddToScheme, hsv1beta1.AddToScheme},
		},
		kind.WithClusterInitializers(clusterInitializers),
	)

	return cluster
}

// Creates the local development cluster.
func (c *Cluster) create(ctx context.Context) error {
	self := run.Meth(c, c.create)
	deps := []run.Dependency{c} // cardboard's internal cluster magic
	if c.registryHostPort != 0 {
		deps = append(deps, run.Meth1(c, c.loadImages, c.registryHostPort))
	}
	return mgr.SerialDeps(ctx, self, deps...)
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
		run.Fn3(pushImage, "package-operator-manager", registry, runtime.GOARCH),
		run.Fn3(pushImage, "package-operator-webhook", registry, runtime.GOARCH),
		run.Fn3(pushImage, "remote-phase-manager", registry, runtime.GOARCH),
		run.Fn3(pushImage, "test-stub", registry, runtime.GOARCH),
	); err != nil {
		return err
	}

	if err := os.Setenv("PKO_REPOSITORY_HOST", hostPort); err != nil {
		return err
	}

	if err := mgr.ParallelDeps(ctx, self,
		run.Fn2(pushPackage, "test-stub", registry),
		run.Fn2(pushPackage, "test-stub-multi", registry),
		run.Fn2(pushPackage, "test-stub-cel", registry),
		run.Fn2(pushPackage, "package-operator", registry),
	); err != nil {
		return err
	}

	return os.Unsetenv("PKO_REPOSITORY_HOST")
}
