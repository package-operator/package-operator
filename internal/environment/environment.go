// The environment package contains probing functionality to
// detect information on the Kubernetes cluster environment.
package environment

import (
	"context"
	stdErrors "errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/version"
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

type Sinker interface {
	SetEnvironment(env *manifestsv1alpha1.PackageEnvironment)
}

type serverVersionDiscoverer interface {
	ServerVersion() (*version.Info, error)
}

func ImplementsSinker(i []interface{}) []Sinker {
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

	sinks []Sinker
}

func NewManager(
	client client.Client,
	discoveryClient serverVersionDiscoverer,
) *Manager {
	return &Manager{
		client:          client,
		discoveryClient: discoveryClient,
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
	log := logr.FromContextOrDiscard(ctx)

	env, err := m.probe(ctx)
	if err != nil {
		return err
	}
	log.Info("detected environment", "environment", env)

	for _, s := range m.sinks {
		s.SetEnvironment(env)
	}
	return nil
}

func (m *Manager) probe(ctx context.Context) (
	env *manifestsv1alpha1.PackageEnvironment, err error,
) {
	env = &manifestsv1alpha1.PackageEnvironment{}

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

	switch {
	case meta.IsNoMatchError(err) || errors.IsNotFound(err) || discovery.IsGroupDiscoveryFailedError(stdErrors.Unwrap(err)):
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
	if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
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

	return &manifestsv1alpha1.PackageEnvironmentProxy{
		HTTPProxy:  httpProxy,
		HTTPSProxy: httpsProxy,
		NoProxy:    noProxy,
	}, true, nil
}

var _ Sinker = (*Sink)(nil)

type Sink struct {
	env  *manifestsv1alpha1.PackageEnvironment
	lock sync.RWMutex
}

func (s *Sink) SetEnvironment(env *manifestsv1alpha1.PackageEnvironment) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.env = env.DeepCopy()
}

func (s *Sink) GetEnvironment() *manifestsv1alpha1.PackageEnvironment {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.env.DeepCopy()
}
