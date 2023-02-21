package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	pkoapis "package-operator.run/apis"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/controllers/adoption"
	"package-operator.run/package-operator/internal/controllers/hostedclusters"
	hypershiftv1beta1 "package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/package-operator/internal/controllers/objectdeployments"
	"package-operator.run/package-operator/internal/controllers/objectsetphases"
	"package-operator.run/package-operator/internal/controllers/objectsets"
	"package-operator.run/package-operator/internal/controllers/packages"
	"package-operator.run/package-operator/internal/dynamiccache"
	"package-operator.run/package-operator/internal/metrics"
	"package-operator.run/package-operator/internal/packages/packagedeploy"
	"package-operator.run/package-operator/internal/packages/packageimport"
)

type opts struct {
	metricsAddr             string
	pprofAddr               string
	namespace               string
	managerImage            string
	selfBootstrap           string
	enableLeaderElection    bool
	probeAddr               string
	printVersion            bool
	copyTo                  string
	loadPackage             string
	remotePhasePackageImage string
}

const (
	metricsAddrFlagDescription  = "The address the metric endpoint binds to."
	pprofAddrFlagDescription    = "The address the pprof web endpoint binds to."
	namespaceFlagDescription    = "The namespace the operator is deployed into."
	managerImageFlagDescription = "Image package operator is deployed with." +
		" e.g. quay.io/package-operator/package-operator-manager"
	leaderElectionFlagDescription = "Enable leader election for controller manager. " +
		"Enabling this will ensure there is only one active controller manager."
	probeAddrFlagDescription   = "The address the probe endpoint binds to."
	versionFlagDescription     = "print version information and exit."
	copyToFlagDescription      = "(internal) copy this binary to a new location"
	loadPackageFlagDescription = "(internal) runs the package-loader sub-component" +
		" to load a package mounted at /package"
	selfBootstrapFlagDescription = "(internal) bootstraps Package Operator" +
		" with Package Operator using the given Package Operator Package Image"
	remotePhasePackageImageFlagDescription = "Image pointing to a package operator remote phase package. " +
		"This image is used with the HyperShift integration to spin up the remote-phase-manager for every HostedCluster"
	hyperShiftPollInterval = 10 * time.Second
)

func main() {
	var opts opts
	flag.StringVar(&opts.metricsAddr, "metrics-addr", ":8080", metricsAddrFlagDescription)
	flag.StringVar(&opts.pprofAddr, "pprof-addr", "", pprofAddrFlagDescription)
	flag.StringVar(&opts.namespace, "namespace", os.Getenv("PKO_NAMESPACE"), namespaceFlagDescription)
	flag.StringVar(&opts.managerImage, "manager-image", os.Getenv("PKO_IMAGE"), managerImageFlagDescription)
	flag.BoolVar(&opts.enableLeaderElection, "enable-leader-election", false, leaderElectionFlagDescription)
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081", probeAddrFlagDescription)
	flag.BoolVar(&opts.printVersion, "version", false, versionFlagDescription)
	flag.StringVar(&opts.copyTo, "copy-to", "", copyToFlagDescription)
	flag.StringVar(&opts.loadPackage, "load-package", "", loadPackageFlagDescription)
	flag.StringVar(&opts.selfBootstrap, "self-bootstrap", "", selfBootstrapFlagDescription)
	flag.StringVar(&opts.remotePhasePackageImage, "remote-phase-package-image",
		os.Getenv("PKO_REMOTE_PHASE_PACKAGE_IMAGE"), remotePhasePackageImageFlagDescription)
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
	if err := hypershiftv1beta1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := apiextensions.AddToScheme(scheme); err != nil {
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

	if len(opts.selfBootstrap) > 0 {
		if err := runBootstrap(setupLog, scheme, opts); err != nil {
			setupLog.Error(err, "unable to run self-bootstrap")
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

	return os.Chmod(destFile.Name(), 0o755)
}

const packageFolderPath = "/package"

func runLoader(scheme *runtime.Scheme, packageKey client.ObjectKey) error {
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	var packageDeployer *packagedeploy.PackageDeployer
	if len(packageKey.Namespace) > 0 {
		// Package API
		packageDeployer = packagedeploy.NewPackageDeployer(c, scheme)
	} else {
		// ClusterPackage API
		packageDeployer = packagedeploy.NewClusterPackageDeployer(c, scheme)
	}

	ctx := logr.NewContext(context.Background(), ctrl.Log.WithName("package-loader"))

	files, err := packageimport.Folder(ctx, packageFolderPath)
	if err != nil {
		return err
	}

	if err := packageDeployer.Load(ctx, packageKey, files); err != nil {
		return err
	}
	return nil
}

//nolint:maintidx
func runManager(log logr.Logger, scheme *runtime.Scheme, opts opts) error {
	controllerLog := ctrl.Log.WithName("controllers")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         opts.metricsAddr,
		HealthProbeBindAddress:     opts.probeAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.enableLeaderElection,
		LeaderElectionID:           "8a4hp84a6s.package-operator-lock",
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
	if err = objectsets.NewObjectSetController(
		mgr.GetClient(),
		controllerLog.WithName("ObjectSet"),
		mgr.GetScheme(), dc, recorder,
		mgr.GetRESTMapper(),
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSet: %w", err)
	}

	if err = objectsets.NewClusterObjectSetController(
		mgr.GetClient(),
		controllerLog.WithName("ClusterObjectSet"),
		mgr.GetScheme(), dc, recorder,
		mgr.GetRESTMapper(),
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectSet: %w", err)
	}

	// ObjectSetPhase for "default" class
	const defaultObjectSetPhaseClass = "default"
	if err = objectsetphases.NewSameClusterObjectSetPhaseController(
		controllerLog.WithName("ObjectSetPhase"),
		mgr.GetScheme(), dc, defaultObjectSetPhaseClass, mgr.GetClient(),
		mgr.GetRESTMapper(),
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ObjectSetPhase: %w", err)
	}

	if err = objectsetphases.NewSameClusterClusterObjectSetPhaseController(
		controllerLog.WithName("ClusterObjectSetPhase"),
		mgr.GetScheme(), dc, defaultObjectSetPhaseClass, mgr.GetClient(),
		mgr.GetRESTMapper(),
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectSetPhase: %w", err)
	}

	// Object deployment controller
	if err = (objectdeployments.NewObjectDeploymentController(
		mgr.GetClient(), controllerLog.WithName("ObjectDeployment"),
		mgr.GetScheme(),
	)).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ObjectDeployment: %w", err)
	}

	// Cluster Object deployment controller
	if err = (objectdeployments.NewClusterObjectDeploymentController(
		mgr.GetClient(), controllerLog.WithName("ClusterObjectDeployment"),
		mgr.GetScheme(),
	)).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ClusterObjectDeployment: %w", err)
	}

	if err = packages.NewPackageController(
		mgr.GetClient(), controllerLog.WithName("Package"), mgr.GetScheme(),
		opts.namespace, opts.managerImage,
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for Package: %w", err)
	}

	if err = packages.NewClusterPackageController(
		mgr.GetClient(), controllerLog.WithName("ClusterPackage"), mgr.GetScheme(),
		opts.namespace, opts.managerImage,
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ClusterPackage: %w", err)
	}

	// Adoption
	// DynamicCache that is not constrained by the DynamicCache label.
	unconstrainedDynamicCache := dynamiccache.NewCache(
		mgr.GetConfig(), mgr.GetScheme(), mgr.GetRESTMapper(), recorder)
	if err = adoption.NewAdoptionController(
		mgr.GetClient(), controllerLog.WithName("Adoption"), unconstrainedDynamicCache, mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for Adoption: %w", err)
	}

	if err = adoption.NewClusterAdoptionController(
		mgr.GetClient(), controllerLog.WithName("ClusterAdoption"), unconstrainedDynamicCache, mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller for ClusterAdoption: %w", err)
	}

	// Probe for HyperShift API
	hostedClusterGVK := hypershiftv1beta1.GroupVersion.WithKind("HostedCluster")
	_, err = mgr.GetRESTMapper().RESTMapping(hostedClusterGVK.GroupKind(), hostedClusterGVK.Version)
	switch {
	case err == nil:
		// HyperShift HostedCluster API is present on the cluster
		// Auto-Enable HyperShift integration controller:
		controllerLog.Info("detected HostedCluster API, enabling HyperShift integration")
		if err = hostedclusters.NewHostedClusterController(
			mgr.GetClient(),
			controllerLog.WithName("HostedCluster"),
			mgr.GetScheme(),
			opts.remotePhasePackageImage,
		).SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create controller for HostedCluster: %w", err)
		}
	case meta.IsNoMatchError(err):
		ticker := clock.RealClock{}.NewTicker(hyperShiftPollInterval)
		if err := mgr.Add(newHypershift(controllerLog.WithName("HyperShift"), mgr.GetRESTMapper(), ticker)); err != nil {
			return fmt.Errorf("add hypershift checker: %w", err)
		}
	default:
		return fmt.Errorf("hypershiftv1beta1 probing: %w", err)
	}

	cleanupClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return fmt.Errorf("create pod cleanup client: %w", err)
	}

	if err := mgr.Add(newCleaner(cleanupClient, opts.namespace)); err != nil {
		return fmt.Errorf("add hypershift checker: %w", err)
	}

	log.Info("starting manager")

	err = mgr.Start(ctrl.SetupSignalHandler())
	switch {
	case err == nil || errors.Is(err, ErrHypershiftAPIPostSetup):
		return nil
	default:
		return fmt.Errorf("problem running manager: %w", err)
	}
}
