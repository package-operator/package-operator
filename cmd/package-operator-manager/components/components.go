package components

import (
	"fmt"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	"go.uber.org/dig"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	apis "package-operator.run/apis"
	"package-operator.run/internal/constants"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/internal/dynamiccache"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/metrics"
)

// Returns a new pre-configured DI container.
func NewComponents() (*dig.Container, error) {
	container := dig.New()
	providers := []any{
		ProvideScheme, ProvideRestConfig, ProvideManager,
		ProvideMetricsRecorder, ProvideDynamicCache,
		ProvideUncachedClient, ProvideOptions, ProvideLogger,
		ProvideRegistry, ProvideDiscoveryClient, ProvideEnvironmentManager,

		// -----------
		// Controllers
		// -----------

		// ObjectSet
		ProvideObjectSetController, ProvideClusterObjectSetController,
		// ObjectSetPhase
		ProvideObjectSetPhaseController, ProvideClusterObjectSetPhaseController,
		// ObjectDeployment
		ProvideObjectDeploymentController, ProvideClusterObjectDeploymentController,
		// Package
		ProvidePackageController, ProvideClusterPackageController,
		// ObjectTemplate
		ProvideObjectTemplateController, ProvideClusterObjectTemplateController,

		// HostedCluster
		ProvideHostedClusterController,

		// SecretSync
		ProvideSecretSyncController,
	}
	for _, p := range providers {
		if err := container.Provide(p); err != nil {
			return nil, err
		}
	}
	return container, nil
}

func ProvideLogger() logr.Logger {
	return ctrl.Log
}

func ProvideScheme() (*runtime.Scheme, error) {
	schemeBuilder := runtime.SchemeBuilder{
		scheme.AddToScheme,
		apis.AddToScheme,
		hypershiftv1beta1.AddToScheme,
		apiextensionsv1.AddToScheme,
		apiextensions.AddToScheme,
		configv1.AddToScheme,
	}
	scheme := runtime.NewScheme()
	if err := schemeBuilder.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func ProvideRestConfig() (*rest.Config, error) {
	return ctrl.GetConfig()
}

func ProvideManager(
	scheme *runtime.Scheme,
	restConfig *rest.Config,
	opts Options,
) (ctrl.Manager, error) {
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                     scheme,
		Metrics:                    server.Options{BindAddress: opts.MetricsAddr},
		HealthProbeBindAddress:     opts.ProbeAddr,
		WebhookServer:              webhook.NewServer(webhook.Options{Port: 9443}),
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.EnableLeaderElection,
		LeaderElectionNamespace:    opts.Namespace,
		LeaderElectionID:           "8a4hp84a6s.package-operator-lock",
		MapperProvider:             apiutil.NewDynamicRESTMapper,
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				// TODO: remove this, we don't unpack via jobs anymore.
				// We create Jobs to unpack package images.
				// Limit caches to only contain Jobs that we create ourselves.
				&batchv1.Job{}: {
					Label: labels.SelectorFromSet(labels.Set{
						constants.DynamicCacheLabel: "True",
					}),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Health and Ready checks
	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up ready check: %w", err)
	}

	// PPROF
	if err := registerPPROF(mgr, opts.PPROFAddr); err != nil {
		return nil, err
	}
	return mgr, nil
}

func ProvideMetricsRecorder() *metrics.Recorder {
	recorder := metrics.NewRecorder()
	recorder.Register()
	return recorder
}

func ProvideDynamicCache(
	mgr ctrl.Manager,
	recorder *metrics.Recorder,
) (*dynamiccache.Cache, error) {
	dc := dynamiccache.NewCache(
		mgr.GetConfig(), mgr.GetScheme(), mgr.GetRESTMapper(), recorder,
		dynamiccache.SelectorsByGVK{
			// Only cache objects with our label selector,=
			// so we prevent our caches from exploding!
			schema.GroupVersionKind{}: dynamiccache.Selector{
				Label: labels.SelectorFromSet(labels.Set{
					constants.DynamicCacheLabel: "True",
				}),
			},
		})
	return dc, nil
}

type UncachedClient struct{ client.Client }

func ProvideUncachedClient(
	restConfig *rest.Config, scheme *runtime.Scheme,
) (UncachedClient, error) {
	uncachedClient, err := client.New(
		restConfig,
		client.Options{
			Scheme: scheme,
		})
	if err != nil {
		return UncachedClient{},
			fmt.Errorf("unable to set up uncached client: %w", err)
	}
	return UncachedClient{uncachedClient}, nil
}

func ProvideDiscoveryClient(restConfig *rest.Config) (
	discovery.DiscoveryInterface, error,
) {
	return discovery.NewDiscoveryClientForConfig(restConfig)
}

func ProvideEnvironmentManager(
	client UncachedClient,
	discoveryClient discovery.DiscoveryInterface,
	mgr ctrl.Manager,
) *environment.Manager {
	return environment.NewManager(
		client, discoveryClient, mgr.GetRESTMapper())
}
