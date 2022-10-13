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

	"package-operator.run/package-operator/internal/metrics"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	pkoapis "package-operator.run/apis"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/controllers/objectdeployments"
	"package-operator.run/package-operator/internal/controllers/objectsetphases"
	"package-operator.run/package-operator/internal/controllers/objectsets"
	"package-operator.run/package-operator/internal/dynamiccache"
)

type opts struct {
	metricsAddr          string
	pprofAddr            string
	namespace            string
	enableLeaderElection bool
	probeAddr            string
	printVersion         bool
}

func main() {
	var opts opts
	flag.StringVar(&opts.metricsAddr, "metrics-addr", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&opts.pprofAddr, "pprof-addr", "",
		"The address the pprof web endpoint binds to.")
	flag.StringVar(&opts.namespace, "namespace", os.Getenv("PKO_NAMESPACE"),
		"The namespace the operator is deployed into.")
	flag.BoolVar(&opts.enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&opts.printVersion, "version", false, "print version information and exit.")
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

		fmt.Fprintln(os.Stderr, version)
		os.Exit(2)
	}

	if err := run(setupLog, scheme, opts); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func run(log logr.Logger, scheme *runtime.Scheme, opts opts) error {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         opts.metricsAddr,
		HealthProbeBindAddress:     opts.probeAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.enableLeaderElection,
		LeaderElectionID:           "8a4hp84a6s.package-operator-lock",
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
				s.Close()
				return nil
			}
		}))
		if err != nil {
			return fmt.Errorf("unable to create pprof server: %w", err)
		}
	}

	// Create metrics recorder
	recorder := metrics.NewRecorder()
	recorder.Register()

	// DynamicCache
	dc := dynamiccache.NewCache(
		mgr.GetConfig(), mgr.GetScheme(), mgr.GetRESTMapper(), recorder,
		dynamiccache.SelectorsByGVK{
			// Only cache objects with our label selector,
			// so we prevent our caches from exploding!
			schema.GroupVersionKind{}: dynamiccache.Selector{
				Label: labels.SelectorFromSet(labels.Set{
					controllers.DynamicCacheLabel: "True",
				}),
			},
		})

	// ObjectSet
	if err = (objectsets.NewObjectSetController(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ObjectSet"),
		mgr.GetScheme(), dc, recorder,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSet: %w", err)
	}
	if err = (objectsets.NewClusterObjectSetController(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ClusterObjectSet"),
		mgr.GetScheme(), dc, recorder,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectSet: %w", err)
	}

	// ObjectSetPhase for "default" class
	const defaultObjectSetPhaseClass = "default"
	if err = (objectsetphases.NewSameClusterObjectSetPhaseController(
		ctrl.Log.WithName("controllers").WithName("ObjectSetPhase"),
		mgr.GetScheme(), dc, defaultObjectSetPhaseClass, mgr.GetClient(),
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSetPhase: %w", err)
	}
	if err = (objectsetphases.NewSameClusterClusterObjectSetPhaseController(
		ctrl.Log.WithName("controllers").WithName("ClusterObjectSetPhase"),
		mgr.GetScheme(), dc, defaultObjectSetPhaseClass, mgr.GetClient(),
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectSetPhase: %w", err)
	}
	// Object deployment controller
	if err = (objectdeployments.NewObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ObjectDeployment"),
		mgr.GetScheme(),
	)).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ObjectDeployment: %w", err)
	}

	// Cluster Object deployment controller
	if err = (objectdeployments.NewClusterObjectDeploymentController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterObjectDeployment"),
		mgr.GetScheme(),
	)).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectDeployment: %w", err)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}
