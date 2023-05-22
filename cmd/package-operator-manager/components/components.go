package components

import (
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/dig"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	pkoapis "package-operator.run/apis"
	"package-operator.run/package-operator/internal/controllers"
	hypershiftv1beta1 "package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/package-operator/internal/dynamiccache"
	"package-operator.run/package-operator/internal/metrics"
)

// Returns a new pre-configured DI container.
func NewComponents() (*dig.Container, error) {
	container := dig.New()
	if err := container.Provide(ProvideScheme); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideRestConfig); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideManager); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideMetricsRecorder); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideDynamicCache); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideUncachedClient); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideOptions); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideLogger); err != nil {
		return nil, err
	}
	if err := container.Provide(ProvideRegistry); err != nil {
		return nil, err
	}

	// -----------
	// Controllers
	// -----------

	// ObjectSet
	if err := container.Provide(
		ProvideObjectSetController); err != nil {
		return nil, err
	}
	if err := container.Provide(
		ProvideClusterObjectSetController); err != nil {
		return nil, err
	}

	// ObjectSetPhase
	if err := container.Provide(
		ProvideObjectSetPhaseController); err != nil {
		return nil, err
	}
	if err := container.Provide(
		ProvideClusterObjectSetPhaseController); err != nil {
		return nil, err
	}

	// ObjectDeployment
	if err := container.Provide(
		ProvideObjectDeploymentController); err != nil {
		return nil, err
	}
	if err := container.Provide(
		ProvideClusterObjectDeploymentController); err != nil {
		return nil, err
	}

	// Package
	if err := container.Provide(
		ProvidePackageController); err != nil {
		return nil, err
	}
	if err := container.Provide(
		ProvideClusterPackageController); err != nil {
		return nil, err
	}

	// ObjectTemplate
	if err := container.Provide(
		ProvideObjectTemplateController); err != nil {
		return nil, err
	}
	if err := container.Provide(
		ProvideClusterObjectTemplateController); err != nil {
		return nil, err
	}

	// HostedCluster
	if err := container.Provide(
		ProvideHostedClusterController); err != nil {
		return nil, err
	}

	return container, nil
}

func ProvideLogger() logr.Logger {
	return ctrl.Log
}

func ProvideScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	schemeBuilder := runtime.SchemeBuilder{
		clientgoscheme.AddToScheme,
		pkoapis.AddToScheme,
		hypershiftv1beta1.AddToScheme,
		apiextensionsv1.AddToScheme,
		apiextensions.AddToScheme,
	}
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
		MetricsBindAddress:         opts.MetricsAddr,
		HealthProbeBindAddress:     opts.ProbeAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.EnableLeaderElection,
		LeaderElectionID:           "8a4hp84a6s.package-operator-lock",
		MapperProvider: func(c *rest.Config) (meta.RESTMapper, error) {
			return apiutil.NewDynamicRESTMapper(c, apiutil.WithLazyDiscovery)
		},
		NewCache: cache.BuilderWithOptions(cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				// We create Jobs to unpack package images.
				// Limit caches to only contain Jobs that we create ourselves.
				&batchv1.Job{}: {
					Label: labels.SelectorFromSet(labels.Set{
						controllers.DynamicCacheLabel: "True",
					}),
				},
			},
		}),
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
					controllers.DynamicCacheLabel: "True",
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

func DiscoveryClient(restConfig *rest.Config) (
	discovery.DiscoveryInterface, error,
) {
	return discovery.NewDiscoveryClientForConfig(restConfig)
}
