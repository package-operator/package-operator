// The environment package contains probing functionality to
// detect information on the Kubernetes cluster environment.
package environment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"package-operator.run/internal/apis/manifests"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

var _ manager.Runnable = (*Manager)(nil)

const (
	environmentProbeInterval    = 1 * time.Minute
	openShiftClusterVersionName = "version"
	openShiftProxyName          = "cluster"

	// Special ConfigMap describing a managed OpenShift cluster.
	managedOpenShiftCMName      = "osd-cluster-metadata"
	managedOpenShiftCMNamespace = "openshift-config"
)

type Sinker interface {
	SetEnvironment(env *manifests.PackageEnvironment)
}

type serverVersionDiscoverer interface {
	ServerVersion() (*version.Info, error)
}

type restMapper interface {
	RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error)
}

func ImplementsSinker(i []any) []Sinker {
	var envSinks []Sinker
	for _, c := range i {
		envSink, ok := c.(Sinker)
		if ok {
			envSinks = append(envSinks, envSink)
		}
	}
	return envSinks
}

type Manager struct {
	client          client.Client
	discoveryClient serverVersionDiscoverer
	restMapper      restMapper

	sinks []Sinker
}

func NewManager(
	client client.Client, // should be of the uncached variety
	discoveryClient serverVersionDiscoverer,
	restMapper restMapper,
) *Manager {
	return &Manager{
		client:          client,
		discoveryClient: discoveryClient,
		restMapper:      restMapper,
	}
}

func (m *Manager) Init(ctx context.Context, sinks []Sinker) error {
	m.sinks = sinks
	return m.do(ctx)
}

// Continuously updates the environment information.
func (m *Manager) Start(ctx context.Context) error {
	t := time.NewTicker(environmentProbeInterval)
	defer t.Stop()

	// periodically re-probe environment until context is closed
	for {
		select {
		case <-t.C:
			if err := m.do(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (m *Manager) do(ctx context.Context) error {
	// Get logger using ctrl.LoggerFrom, which will provide a logger even if ctx doesn't have one
	log := ctrl.LoggerFrom(ctx)

	env, err := m.probe(ctx)
	if err != nil {
		return err
	}

	log.Info("detected environment", "environment", env)

	for _, sink := range m.sinks {
		sink.SetEnvironment(env)
	}

	return nil
}

func (m *Manager) probe(ctx context.Context) (
	env *manifests.PackageEnvironment, err error,
) {
	env = &manifests.PackageEnvironment{}
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
		proxy, hasProxy, err := m.openShiftProxyEnvironment(ctx)
		if err != nil {
			return env, fmt.Errorf("getting OpenShift Proxy settings: %w", err)
		}

		if hasProxy {
			env.Proxy = proxy
		}
	}

	hyperShiftEnv, _, err := m.hyperShiftEnvironment()
	if err != nil {
		return env, fmt.Errorf("getting HyperShift env: %w", err)
	}
	env.HyperShift = hyperShiftEnv

	return env, nil
}

func (m *Manager) kubernetesEnvironment(_ context.Context) (
	kubeEnv manifests.PackageEnvironmentKubernetes, err error,
) {
	serverVersion, err := m.discoveryClient.ServerVersion()
	if err != nil {
		return kubeEnv, fmt.Errorf("getting server version from discovery API: %w", err)
	}
	kubeEnv.Version = serverVersion.GitVersion
	return kubeEnv, nil
}

func (m *Manager) openShiftEnvironment(ctx context.Context) (
	openShiftEnv *manifests.PackageEnvironmentOpenShift, isOpenShift bool, err error,
) {
	clusterVersion := &configv1.ClusterVersion{}
	err = m.client.Get(ctx, client.ObjectKey{
		Name: openShiftClusterVersionName,
	}, clusterVersion)

	switch {
	case meta.IsNoMatchError(err) ||
		apimachineryerrors.IsNotFound(err) ||
		discovery.IsGroupDiscoveryFailedError(errors.Unwrap(err)):
		// API not registered in cluster
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}

	var openShiftVersion string
	for _, update := range clusterVersion.Status.History {
		if update.State == configv1.CompletedUpdate {
			// obtain the version from the last completed update
			openShiftVersion = update.Version
			break
		}
	}

	openShiftEnv = &manifests.PackageEnvironmentOpenShift{
		Version: openShiftVersion,
	}
	if managedOpenShift, isManagedOpenShift, err := m.managedOpenShiftEnvironment(ctx); err != nil {
		return nil, false, err
	} else if isManagedOpenShift {
		openShiftEnv.Managed = managedOpenShift
	}

	return openShiftEnv, true, nil
}

func (m *Manager) openShiftProxyEnvironment(ctx context.Context) (
	openShiftEnv *manifests.PackageEnvironmentProxy, hasProxy bool, err error,
) {
	proxy := &configv1.Proxy{}
	err = m.client.Get(ctx, client.ObjectKey{
		Name: openShiftProxyName,
	}, proxy)
	if meta.IsNoMatchError(err) || apimachineryerrors.IsNotFound(err) {
		// API not registered in cluster or no proxy config.
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf(
			"getting OpenShift ClusterVersion: %w", err)
	}

	var (
		httpProxy  = proxy.Status.HTTPProxy
		httpsProxy = proxy.Status.HTTPSProxy
		noProxy    = proxy.Status.NoProxy
	)

	if httpProxy == "" && httpsProxy == "" && noProxy == "" {
		return nil, false, nil
	}

	return &manifests.PackageEnvironmentProxy{
		HTTPProxy:  httpProxy,
		HTTPSProxy: httpsProxy,
		NoProxy:    noProxy,
	}, true, nil
}

func (m *Manager) managedOpenShiftEnvironment(ctx context.Context) (
	managedOpenShift *manifests.PackageEnvironmentManagedOpenShift,
	isManagedOpenShift bool,
	err error,
) {
	cm := &corev1.ConfigMap{}
	err = m.client.Get(ctx, client.ObjectKey{
		Name:      managedOpenShiftCMName,
		Namespace: managedOpenShiftCMNamespace,
	}, cm)
	if apimachineryerrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf(
			"getting managed OpenShift ConfigMap: %w", err)
	}
	return &manifests.PackageEnvironmentManagedOpenShift{
		Data: cm.Data,
	}, true, nil
}

var hostedClusterGVK = hypershiftv1beta1.GroupVersion.
	WithKind("HostedCluster")

func (m *Manager) hyperShiftEnvironment() (
	hyperShift *manifests.PackageEnvironmentHyperShift, isHyperShift bool, err error,
) {
	// Probe for HyperShift API
	_, err = m.restMapper.
		RESTMapping(hostedClusterGVK.GroupKind(), hostedClusterGVK.Version)
	switch {
	case err == nil:
		// HyperShift HostedCluster API is present on the cluster.
		return &manifests.PackageEnvironmentHyperShift{}, true, nil

	case meta.IsNoMatchError(err) ||
		apimachineryerrors.IsNotFound(err) ||
		discovery.IsGroupDiscoveryFailedError(errors.Unwrap(err)):
		// HyperShift HostedCluster API is NOT present on the cluster.
		return nil, false, nil
	}

	// Error probing.
	return nil, false, fmt.Errorf("hypershiftv1beta1 probing: %w", err)
}

var _ Sinker = (*Sink)(nil)

type Sink struct {
	client client.Client
	env    *manifests.PackageEnvironment
	lock   sync.RWMutex
}

func NewSink(c client.Client) *Sink {
	return &Sink{client: c}
}

func (s *Sink) SetEnvironment(env *manifests.PackageEnvironment) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.env = env.DeepCopy()
}

func (s *Sink) GetEnvironment(ctx context.Context, namespace string) (*manifests.PackageEnvironment, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	env := s.env.DeepCopy()

	if len(namespace) == 0 || env.HyperShift == nil {
		return env, nil
	}

	// Lookup HostedCluster
	hostedClusterList := &hypershiftv1beta1.HostedClusterList{}
	if err := s.client.List(ctx, hostedClusterList); err != nil {
		return nil, fmt.Errorf("listing HostedClusters: %w", err)
	}
	for _, hc := range hostedClusterList.Items {
		hcNamespace := hypershiftv1beta1.HostedClusterNamespace(hc)
		if hcNamespace != namespace {
			continue
		}

		env.HyperShift.HostedCluster = &manifests.PackageEnvironmentHyperShiftHostedCluster{
			TemplateContextObjectMeta: manifests.TemplateContextObjectMeta{
				Name:        hc.Name,
				Namespace:   hc.Namespace,
				Labels:      hc.Labels,
				Annotations: hc.Annotations,
			},
			HostedClusterNamespace: hcNamespace,
			NodeSelector:           hc.Spec.NodeSelector,
		}
		return env, nil
	}

	// No HostedCluster found for this namespace.
	return env, nil
}
