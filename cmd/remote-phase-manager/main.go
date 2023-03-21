package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime/debug"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	pkoapis "package-operator.run/apis"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/controllers/objectsetphases"
	"package-operator.run/package-operator/internal/dynamiccache"
	"package-operator.run/package-operator/internal/metrics"
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
}

const (
	metricsAddrFlagDescription  = "The address the metric endpoint binds to."
	pprofAddrFlagDescription    = "The address the pprof web endpoint binds to."
	namespaceFlagDescription    = "The namespace the operator is deployed into."
	managerImageFlagDescription = "Image package operator is deployed with." +
		" e.g. quay.io/package-operator/package-operator-manager"
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
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	scheme := runtime.NewScheme()
	setupLog := ctrl.Log.WithName("setup")
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := pkoapis.AddToScheme(scheme); err != nil {
		panic(err)
	}

	if opts.printVersion {
		version := "binary compiled without version info"

		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.String()
		}

		_, _ = fmt.Fprintln(os.Stderr, version)
		os.Exit(2)
	}

	if err := run(setupLog, scheme, opts); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func run(log logr.Logger, scheme *runtime.Scheme, opts opts) error {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Namespace:                  opts.namespace,
		Scheme:                     scheme,
		MetricsBindAddress:         opts.metricsAddr,
		HealthProbeBindAddress:     opts.probeAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.enableLeaderElection,
		LeaderElectionID:           "klsdfu452p3.package-operator-lock",
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
				for range errCh {
				} // drain errCh for GC
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
	targetMapper, err := apiutil.NewDiscoveryRESTMapper(targetCfg)
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

	dc := dynamiccache.NewCache(
		targetCfg, scheme, targetMapper, recorder,
		dynamiccache.SelectorsByGVK{
			// Only cache objects with our label selector,
			// so we prevent our caches from exploding!
			schema.GroupVersionKind{}: dynamiccache.Selector{
				Label: labels.SelectorFromSet(labels.Set{
					controllers.DynamicCacheLabel: "True",
				}),
			},
		})

	// Create a client that does not cache resources cluster-wide.
	uncachedClient, err := client.New(
		mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme(), Mapper: mgr.GetRESTMapper()})
	if err != nil {
		return fmt.Errorf("unable to set up uncached client: %w", err)
	}

	managementClusterClient := mgr.GetClient()

	if err = objectsetphases.NewMultiClusterObjectSetPhaseController(
		ctrl.Log.WithName("controllers").WithName("ObjectSetPhase"),
		mgr.GetScheme(), dc, uncachedClient,
		opts.class, managementClusterClient,
		targetClient, targetMapper,
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSetPhase: %w", err)
	}

	if len(opts.namespace) == 0 {
		// Only start the Cluster-Scoped controller, when we are running cluster scoped.
		if err = objectsetphases.NewMultiClusterClusterObjectSetPhaseController(
			ctrl.Log.WithName("controllers").WithName("ClusterObjectSetPhase"),
			mgr.GetScheme(), dc, uncachedClient,
			opts.class, managementClusterClient,
			targetClient, targetMapper,
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
