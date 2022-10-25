package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"package-operator.run/package-operator/internal/metrics"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	pkoapis "package-operator.run/apis"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/controllers/objectdeployments"
	"package-operator.run/package-operator/internal/controllers/objectsetphases"
	"package-operator.run/package-operator/internal/controllers/objectsets"
	"package-operator.run/package-operator/internal/controllers/packages"
	"package-operator.run/package-operator/internal/dynamiccache"
	packageloader "package-operator.run/package-operator/internal/packages"
)

type opts struct {
	metricsAddr          string
	pprofAddr            string
	namespace            string
	image                string
	enableLeaderElection bool
	probeAddr            string
	printVersion         bool
	copyTo               string
	loadPackage          string
}

func main() {
	var opts opts
	flag.StringVar(&opts.metricsAddr, "metrics-addr", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&opts.pprofAddr, "pprof-addr", "",
		"The address the pprof web endpoint binds to.")
	flag.StringVar(&opts.namespace, "namespace", os.Getenv("PKO_NAMESPACE"),
		"The namespace the operator is deployed into.")
	flag.StringVar(&opts.image, "image", os.Getenv("PKO_IMAGE"),
		"Image package operator is deployed with.")
	flag.BoolVar(&opts.enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&opts.printVersion, "version", false, "print version information and exit.")
	flag.StringVar(&opts.copyTo, "copy-to", "", "(internal) copy this binary to a new location")
	flag.StringVar(&opts.loadPackage, "load-package", "", "(internal) runs the package-loader sub-component to load a package mounted at /package")
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

	if len(opts.loadPackage) > 0 {
		namespace, name, found := strings.Cut(opts.loadPackage, string(types.Separator))
		if !found {
			fmt.Fprintln(os.Stderr, "invalid argument to --load-package, expected NamespaceName")
			os.Exit(1)
		}

		packageKey := client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}
		if err := runLoader(scheme, packageKey); err != nil {
			setupLog.Error(err, "unable to run package-loader")
			os.Exit(1)
		}
		return
	}

	if len(opts.copyTo) > 0 {
		if err := runCopyTo(opts.copyTo); err != nil {
			setupLog.Error(err, "unable to run copy-to")
			os.Exit(1)
		}
		return
	}

	if err := runManager(setupLog, scheme, opts); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func runCopyTo(target string) error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("looking up current executable path: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("opening destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return os.Chmod(destFile.Name(), 0755)
}

const packageFolderPath = "/package"

func runLoader(scheme *runtime.Scheme, packageKey client.ObjectKey) error {
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("creating target cluster client: %w", err)
	}

	var packageLoader *packageloader.PackageLoader
	if len(packageKey.Namespace) > 0 {
		// Package API
		packageLoader = packageloader.NewPackageLoader(c, scheme)
	} else {
		// ClusterPackage API
		packageLoader = packageloader.NewClusterPackageLoader(c, scheme)
	}

	ctx := logr.NewContext(context.Background(), ctrl.Log.WithName("package-loader"))
	if err := packageLoader.Load(ctx, packageKey, packageFolderPath); err != nil {
		return err
	}
	return nil
}

func runManager(log logr.Logger, scheme *runtime.Scheme, opts opts) error {
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
			// Only cache objects with our label selector,=
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
		mgr.GetRESTMapper(),
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSet: %w", err)
	}
	if err = (objectsets.NewClusterObjectSetController(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ClusterObjectSet"),
		mgr.GetScheme(), dc, recorder,
		mgr.GetRESTMapper(),
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectSet: %w", err)
	}

	// ObjectSetPhase for "default" class
	const defaultObjectSetPhaseClass = "default"
	if err = (objectsetphases.NewSameClusterObjectSetPhaseController(
		ctrl.Log.WithName("controllers").WithName("ObjectSetPhase"),
		mgr.GetScheme(), dc, defaultObjectSetPhaseClass, mgr.GetClient(),
		mgr.GetRESTMapper(),
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSetPhase: %w", err)
	}
	if err = (objectsetphases.NewSameClusterClusterObjectSetPhaseController(
		ctrl.Log.WithName("controllers").WithName("ClusterObjectSetPhase"),
		mgr.GetScheme(), dc, defaultObjectSetPhaseClass, mgr.GetClient(),
		mgr.GetRESTMapper(),
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

	if err = (packages.NewPackageController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("Package"), mgr.GetScheme(),
		opts.namespace, opts.image,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for Package: %w", err)
	}

	if err = (packages.NewClusterPackageController(
		mgr.GetClient(), ctrl.Log.WithName("controllers").WithName("ClusterPackage"), mgr.GetScheme(),
		opts.namespace, opts.image,
	).SetupWithManager(mgr)); err != nil {
		return fmt.Errorf("unable to create controller for ClusterPackage: %w", err)
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}
	return nil
}
