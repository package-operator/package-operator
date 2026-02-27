package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"pkg.package-operator.run/boxcutter/managedcache"

	apis "package-operator.run/apis"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/controllers/objectsetphases"
	"package-operator.run/internal/metrics"
	"package-operator.run/internal/version"
)

type opts struct {
	metricsAddr                 string
	pprofAddr                   string
	namespace                   string
	enableLeaderElection        bool
	probeAddr                   string
	class                       string
	targetClusterKubeconfigFile string
	printVersion                bool
	logLevel                    int
}

const (
	metricsAddrFlagDescription    = "The address the metric endpoint binds to."
	pprofAddrFlagDescription      = "The address the pprof web endpoint binds to."
	namespaceFlagDescription      = "The namespace the operator is deployed into."
	leaderElectionFlagDescription = "Enable leader election for controller manager. " +
		"Enabling this will ensure there is only one active controller manager."
	probeAddrFlagDescription     = "The address the probe endpoint binds to."
	versionFlagDescription       = "print version information and exit."
	classFlagDescription         = "class of the ObjectSetPhase to work on."
	targetClusterFlagDescription = "Filepath for a kubeconfig for the target cluster."
)

func main() {
	var opts opts
	flag.StringVar(&opts.metricsAddr, "metrics-addr", ":8080", metricsAddrFlagDescription)
	flag.StringVar(&opts.pprofAddr, "pprof-addr", "", pprofAddrFlagDescription)
	flag.StringVar(&opts.namespace, "namespace", os.Getenv("PKO_NAMESPACE"), namespaceFlagDescription)
	flag.BoolVar(&opts.enableLeaderElection, "enable-leader-election", false, leaderElectionFlagDescription)
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081", probeAddrFlagDescription)
	flag.StringVar(&opts.targetClusterKubeconfigFile, "target-cluster-kubeconfig-file", "", targetClusterFlagDescription)
	flag.StringVar(&opts.class, "class", "hosted-cluster", classFlagDescription)
	flag.BoolVar(&opts.printVersion, "version", false, versionFlagDescription)
	defaultLogLevel := -1
	if lvl, err := strconv.Atoi(os.Getenv("LOG_LEVEL")); err == nil {
		defaultLogLevel = lvl
	}
	flag.IntVar(
		&opts.logLevel, "log-level", defaultLogLevel,
		"Log level. Default is -1 (warn). Higher numbers increase verbosity (e.g., 0 = info, 1 = debug)")
	flag.Parse()
	zapLevel := zapcore.Level(-1 * opts.logLevel)
	zapOpts := zap.Options{
		Development: false,
		Level:       zapLevel,
	}
	if opts.printVersion {
		_ = version.Get().Write(os.Stderr)

		os.Exit(2)
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	ourScheme := runtime.NewScheme()
	setupLog := ctrl.Log.WithName("setup")
	schemeBuilder := runtime.SchemeBuilder{
		scheme.AddToScheme,
		apis.AddToScheme,
	}
	if err := schemeBuilder.AddToScheme(ourScheme); err != nil {
		panic(err)
	}

	if err := run(setupLog, ourScheme, opts); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func run(log logr.Logger, scheme *runtime.Scheme, opts opts) error {
	namespaces := map[string]cache.Config{}
	if opts.namespace != "" {
		namespaces[opts.namespace] = cache.Config{}
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Cache:                      cache.Options{DefaultNamespaces: namespaces},
		Metrics:                    server.Options{BindAddress: opts.metricsAddr},
		Scheme:                     scheme,
		HealthProbeBindAddress:     opts.probeAddr,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.enableLeaderElection,
		LeaderElectionID:           "klsdfu452p3.package-operator-lock",
		// Caution: enabling `LeaderElectionReleaseOnCancel` requires us to stop the binary
		// right after the manager ends! (Docstring on the field has more information.)
		LeaderElectionReleaseOnCancel: true,
		// Recommended Leader Election values
		// https://github.com/openshift/enhancements/blob/61581dcd985130357d6e4b0e72b87ee35394bf6e/CONVENTIONS.md#handling-kube-apiserver-disruption
		LeaseDuration: ptr.To(137 * time.Second),
		RenewDeadline: ptr.To(107 * time.Second),
		RetryPeriod:   ptr.To(26 * time.Second),
	})
	if err != nil {
		return fmt.Errorf("creating manager: %w", err)
	}

	// Health and Ready checks
	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	// PPROF
	if len(opts.pprofAddr) > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		s := &http.Server{Addr: opts.pprofAddr, Handler: mux, ReadHeaderTimeout: 1 * time.Second}
		err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			errCh := make(chan error)
			defer func() {
				// drain errCh for GC.
				for range errCh {
				}
			}()
			go func() {
				defer close(errCh)
				errCh <- s.ListenAndServe()
			}()

			select {
			case err := <-errCh:
				return err
			case <-ctx.Done():
				_ = s.Close()
				return nil
			}
		}))
		if err != nil {
			return fmt.Errorf("unable to create pprof server: %w", err)
		}
	}

	targetCfg, err := clientcmd.BuildConfigFromFlags("", opts.targetClusterKubeconfigFile)
	if err != nil {
		return fmt.Errorf("reading target cluster kubeconfig: %w", err)
	}
	targetHTTPClient, err := rest.HTTPClientFor(targetCfg)
	if err != nil {
		return fmt.Errorf("building http client for kubeconfig: %w", err)
	}
	targetMapper, err := apiutil.NewDynamicRESTMapper(targetCfg, targetHTTPClient)
	if err != nil {
		return fmt.Errorf("creating target cluster rest mapper: %w", err)
	}
	targetClient, err := client.New(targetCfg, client.Options{
		Scheme: scheme,
		Mapper: targetMapper,
	})
	if err != nil {
		return fmt.Errorf("creating target cluster client: %w", err)
	}

	// Create metrics recorder
	recorder := metrics.NewRecorder()
	recorder.Register()

	mapper := func(
		_ context.Context, _ client.Object,
		c *rest.Config, o cache.Options,
	) (*rest.Config, cache.Options, error) {
		return c, o, nil
	}

	accessManager := managedcache.NewObjectBoundAccessManager(
		log,
		mapper,
		targetCfg,
		cache.Options{
			Scheme: scheme,
			Mapper: targetMapper,
			DefaultLabelSelector: labels.SelectorFromSet(labels.Set{
				constants.DynamicCacheLabel: "True",
			}),
		},
	)

	if err := mgr.Add(accessManager); err != nil {
		return fmt.Errorf("unable to start cache manager: %w", err)
	}

	metricsCollector := metrics.NewManagedCacheCollector(accessManager, log)
	ctrlmetrics.Registry.MustRegister(metricsCollector)

	managementClusterClient := mgr.GetClient()
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(targetCfg)
	if err != nil {
		return fmt.Errorf("unable to create discovery client: %w", err)
	}

	if err = objectsetphases.NewMultiClusterObjectSetPhaseController(
		ctrl.Log.WithName("controllers").WithName("ObjectSetPhase"),
		mgr.GetScheme(), accessManager,
		opts.class, managementClusterClient,
		targetClient, targetMapper, discoveryClient,
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSetPhase: %w", err)
	}

	if len(opts.namespace) == 0 {
		// Only start the Cluster-Scoped controller, when we are running cluster scoped.
		if err = objectsetphases.NewMultiClusterClusterObjectSetPhaseController(
			ctrl.Log.WithName("controllers").WithName("ClusterObjectSetPhase"),
			mgr.GetScheme(), accessManager,
			opts.class, managementClusterClient,
			targetClient, targetMapper, discoveryClient,
		).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create controller for ClusterObjectSetPhase: %w", err)
		}
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}
