// The environment package contains probing functionality to
// detect information on the Kubernetes cluster environment.
package environment

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var _ manager.Runnable = (*Manager)(nil)

const (
	environmentProbeInterval    = 1 * time.Minute
	openShiftClusterVersionName = "version"
	openShiftProxyName          = "cluster"
)

type Sink interface {
	InjectEnvironment(env manifestsv1alpha1.PackageEnvironment)
}

type Manager struct {
	client          client.Client
	discoveryClient discovery.DiscoveryInterface
	sinks           []Sink
}

func NewManager(
	client client.Client,
	discoveryClient discovery.DiscoveryInterface,
	sinks []Sink,
) *Manager {
	return &Manager{
		client:          client,
		discoveryClient: discoveryClient,
		sinks:           sinks,
	}
}

func (m *Manager) Init(ctx context.Context) error {
	return m.do(ctx)
}

// Continuously updates the environment information.
func (m *Manager) Start(ctx context.Context) error {
	t := time.NewTicker(environmentProbeInterval)
	defer t.Stop()
	for range t.C {
		if err := m.do(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) do(ctx context.Context) error {
	env, err := m.probe(ctx)
	if err != nil {
		return err
	}
	for _, s := range m.sinks {
		s.InjectEnvironment(env)
	}
	return nil
}

func (m *Manager) probe(ctx context.Context) (
	env manifestsv1alpha1.PackageEnvironment, err error,
) {
	kubeEnv, err := m.kubernetesEnvironment(ctx)
	if err != nil {
		return env, fmt.Errorf("getting k8s env: %w", err)
	}
	env.Kubernetes = kubeEnv

	openShiftEnv, isOpenShift, err := m.openShiftEnvironment(ctx)
	if err != nil {
		return env, fmt.Errorf("getting OpenShift env: %w", err)
	}
	env.OpenShift = openShiftEnv

	if isOpenShift {
		proxy, _, err := m.openShiftProxyEnvironment(ctx)
		if err != nil {
			return env, fmt.Errorf("getting OpenShift Proxy settings: %w", err)
		}
		env.Proxy = proxy
	}
	return
}

func (m *Manager) kubernetesEnvironment(_ context.Context) (
	kubeEnv manifestsv1alpha1.PackageEnvironmentKubernetes, err error,
) {
	serverVersion, err := m.discoveryClient.ServerVersion()
	if err != nil {
		return kubeEnv, fmt.Errorf("getting server version from discovery API: %w", err)
	}
	kubeEnv.Version = serverVersion.GitVersion
	return kubeEnv, nil
}

func (m *Manager) openShiftEnvironment(ctx context.Context) (
	openShiftEnv *manifestsv1alpha1.PackageEnvironmentOpenShift, isOpenShift bool, err error,
) {
	clusterVersion := &configv1.ClusterVersion{}
	err = m.client.Get(ctx, client.ObjectKey{
		Name: openShiftClusterVersionName,
	}, clusterVersion)
	if meta.IsNoMatchError(err) {
		// API not registered in cluster
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("getting OpenShift ClusterVersion: %w", err)
	}

	var openShiftVersion string
	for _, update := range clusterVersion.Status.History {
		if update.State == configv1.CompletedUpdate {
			// obtain the version from the last completed update
			openShiftVersion = update.Version
			break
		}
	}

	return &manifestsv1alpha1.PackageEnvironmentOpenShift{
		Version: openShiftVersion,
	}, true, nil
}

func (m *Manager) openShiftProxyEnvironment(ctx context.Context) (
	openShiftEnv *manifestsv1alpha1.PackageEnvironmentProxy, hasProxy bool, err error,
) {
	proxy := &configv1.Proxy{}
	err = m.client.Get(ctx, client.ObjectKey{
		Name: openShiftProxyName,
	}, proxy)
	if meta.IsNoMatchError(err) {
		// API not registered in cluster
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("getting OpenShift ClusterVersion: %w", err)
	}

	return &manifestsv1alpha1.PackageEnvironmentProxy{
		HTTP:  proxy.Status.HTTPProxy,
		HTTPS: proxy.Status.HTTPSProxy,
		No:    proxy.Status.NoProxy,
	}, true, nil
}
