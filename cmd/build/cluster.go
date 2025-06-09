package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	authHostPort int32
}

type clusterConfig struct {
	name                  string
	registryHostOverrides []struct {
		host, endpoint string
	}
	localRegistry *clusterConfigLocalRegistry
	nodeLabels    map[string]string
}

func (cc *clusterConfig) apply(opts ...clusterOption) {
	for _, opt := range opts {
		opt(cc)
	}
}

type clusterOption func(*clusterConfig)

func withNodeLabels(labels map[string]string) clusterOption {
	return func(cc *clusterConfig) {
		cc.nodeLabels = labels
	}
}

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

func withLocalRegistry(hostOverride string, hostPort int32, authHostPort int32) clusterOption {
	return func(cc *clusterConfig) {
		cc.localRegistry = &clusterConfigLocalRegistry{
			hostPort:     hostPort,
			authHostPort: authHostPort,
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
		extraPortMappings = append(extraPortMappings, kindv1alpha4.PortMapping{
			ContainerPort: 5002,
			HostPort:      cfg.localRegistry.authHostPort,
			ListenAddress: "127.0.0.1",
			Protocol:      "TCP",
		})
	}

	cluster.Cluster = kind.NewCluster(cfg.name,
		kind.WithClusterConfig{
			ContainerdConfigPatches: containerdConfigPatches,
			Nodes: []kindv1alpha4.Node{
				{
					Labels:            cfg.nodeLabels,
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

// Create the local Hypershift development environment.
func (c *Cluster) createHostedCluster(ctx context.Context, mgmtCl *Cluster, args []string) error {
	self := run.Meth2(c, c.createHostedCluster, mgmtCl, args)
	if err := mgr.ParallelDeps(ctx, self,
		run.Meth1(mgmtCl, mgmtCl.loadCRDs, []string{}),
		run.Meth1(mgmtCl, mgmtCl.installHypershiftAPIs, []string{}),
		run.Meth(c, c.create),
	); err != nil {
		return err
	}

	// get mgmt cluster clients
	mgmtClients, err := mgmtCl.Clients()
	if err != nil {
		return fmt.Errorf("can't get client for mgmt cluster %s: %w", mgmtCl.Name(), err)
	}
	// get hosted cluster clients
	hstdClients, err := c.Clients()
	if err != nil {
		return fmt.Errorf("can't get client for hosted cluster %s: %w", c.Name(), err)
	}

	// create package-operator-remote-phase-manager ClusterRole in mgmt cluster
	rpmCrPath := filepath.Join("config", "packages", "package-operator", "rbac",
		"package-operator-remote-phase-manager.ClusterRole.yaml")
	if err = mgmtClients.CreateAndWaitFromFiles(ctx, []string{rpmCrPath}); err != nil {
		return fmt.Errorf("can't create remote phase manager ClusterRole in mgmt cluster %s: %w", mgmtCl.Name(), err)
	}

	// get kubeconfig of hosted cluster and replace hostname with cluster IP
	hstdClKubeconfig, err := c.Kubeconfig(true)
	if err != nil {
		return fmt.Errorf("can't get Kubeconfig of hosted cluster %s: %w", c.Name(), err)
	}

	// create namespace
	namespaceName := "default-" + c.Name()
	mgmtNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	if err := mgmtClients.CreateAndWaitForReadiness(ctx, mgmtNs); err != nil {
		return fmt.Errorf("can't create hosted cluster namespace in mgmt cluster %s: %w", mgmtCl.Name(), err)
	}
	hstdNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	if err := hstdClients.CreateAndWaitForReadiness(ctx, hstdNs); err != nil {
		return fmt.Errorf("can't create hosted cluster namespace in hosted cluster %s: %w",
			c.Name(), err)
	}

	// create secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-network-admin-kubeconfig",
			Namespace: namespaceName,
		},
		Data: map[string][]byte{
			"kubeconfig": []byte(hstdClKubeconfig),
		},
	}
	if err := mgmtClients.CreateAndWaitForReadiness(ctx, secret); err != nil {
		return fmt.Errorf("can't create kubeconfig secret in mgmt cluster %s: %w", mgmtCl.Name(), err)
	}

	// create hosted cluster
	hstdClResource := &hsv1beta1.HostedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name(),
			Namespace: "default",
		},
		Spec: hsv1beta1.HostedClusterSpec{
			NodeSelector: map[string]string{
				"hosted-cluster": "node-selector",
			},
		},
	}

	if err := mgmtClients.CreateAndWaitForReadiness(ctx, hstdClResource); err != nil {
		return fmt.Errorf("can't create HostedCluster in mgmt cluster %s: %w", mgmtCl.Name(), err)
	}

	// list all nodes in the management cluster
	nodeList := &corev1.NodeList{}
	if err := mgmtClients.CtrlClient.List(ctx, nodeList); err != nil {
		return fmt.Errorf("can't list nodes in management cluster %s: %w", c.Name(), err)
	}

	// label each node
	for _, node := range nodeList.Items {
		nodeCopy := node.DeepCopy()
		if nodeCopy.Labels == nil {
			nodeCopy.Labels = make(map[string]string)
		}
		nodeCopy.Labels["hosted-cluster"] = "node-selector"
		if err := mgmtClients.CtrlClient.Update(ctx, nodeCopy); err != nil {
			return fmt.Errorf("can't label node %s in hosted cluster %s: %w", node.Name, c.Name(), err)
		}
	}

	hstdClResource.Status.Conditions = []metav1.Condition{
		{
			Type:               hsv1beta1.HostedClusterAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "Success",
			Message:            "HostedCluster is Available (manually set)",
			ObservedGeneration: hstdClResource.GetGeneration(),
			LastTransitionTime: metav1.Now(),
		},
	}
	if err := mgmtClients.CtrlClient.Status().Update(ctx, hstdClResource); err != nil {
		return fmt.Errorf("can't apply HostedCluster status in mgmt cluster %s: %w", mgmtCl.Name(), err)
	}

	return nil
}

// Destroys the local development cluster.
func (c *Cluster) destroy(ctx context.Context) error {
	return c.Destroy(ctx)
}

// Install the Hypershift HostedCluster API in the local development cluster.
func (c *Cluster) installHypershiftAPIs(ctx context.Context, _ []string) error {
	self := run.Meth1(c, c.installHypershiftAPIs, []string{})
	if err := mgr.ParallelDeps(ctx, self,
		run.Meth(c, c.create),
	); err != nil {
		return err
	}

	clClients, err := c.Clients()
	if err != nil {
		return fmt.Errorf("getting cluster %s client: %w", c.Name(), err)
	}

	// install hosted cluster CRD into mgmt cluster
	hcCrdPath := filepath.Join("integration", "package-operator", "testdata", "hostedclusters.crd.yaml")

	if err = clClients.CreateAndWaitFromFiles(ctx, []string{hcCrdPath}); err != nil {
		return fmt.Errorf("applying HostedCluster CRD to cluster %s: %w", c.Name(), err)
	}
	return nil
}

// Load images into the local development cluster.
func (c *Cluster) loadImages(ctx context.Context, registryPort int32) error {
	self := run.Meth1(c, c.loadImages, registryPort)

	hostPort := fmt.Sprintf("localhost:%d", registryPort)
	registry := localRegistry(hostPort)

	if err := mgr.ParallelDeps(ctx, self,
		run.Fn3(pushImage, "package-operator-manager", registry, runtime.GOARCH),
		run.Fn3(pushImage, "remote-phase-manager", registry, runtime.GOARCH),
		run.Fn3(pushImage, "test-stub", registry, runtime.GOARCH),
	); err != nil {
		return err
	}

	// The test stub is reused for image prefix overrides test but needs to be named differently.
	// So mirror it to another tag.
	if err := mirrorImage("test-stub", "src/test-stub-mirror", registry); err != nil {
		return err
	}

	if err := os.Setenv("PKO_REPOSITORY_HOST", hostPort); err != nil {
		return err
	}

	if err := mgr.ParallelDeps(ctx, self,
		run.Fn2(pushPackage, "test-stub", registry),
		run.Fn2(pushPackage, "test-stub-multi", registry),
		run.Fn2(pushPackage, "test-stub-cel", registry),
		run.Fn2(pushPackage, "test-stub-pause", registry),
		run.Fn2(pushPackage, "package-operator", registry),
		run.Fn2(pushPackage, "test-stub-image-prefix-override", registry),
	); err != nil {
		return err
	}

	return os.Unsetenv("PKO_REPOSITORY_HOST")
}

// Load CRDs into the local development cluster.
func (c *Cluster) loadCRDs(ctx context.Context, args []string) error {
	self := run.Meth1(c, c.loadCRDs, args)
	if err := mgr.ParallelDeps(ctx, self,
		run.Meth(generate, generate.code),
		run.Meth(c, c.create),
	); err != nil {
		return err
	}

	// get cluster clients
	clients, err := c.Clients()
	if err != nil {
		return err
	}

	// load CRDs
	entries, err := os.ReadDir(filepath.Join("config", "crds"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			entryPath := filepath.Join("config", "crds", entry.Name())
			if err = clients.CreateAndWaitFromFiles(ctx, []string{entryPath}); err != nil {
				return err
			}
		}
	}

	return nil
}
