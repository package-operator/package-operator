package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/openshift/addon-operator/internal/metrics"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	aoapis "github.com/openshift/addon-operator/apis"
	addoncontroller "github.com/openshift/addon-operator/internal/controllers/addon"
	aicontroller "github.com/openshift/addon-operator/internal/controllers/addoninstance"
	aocontroller "github.com/openshift/addon-operator/internal/controllers/addonoperator"
)

var (
	scheme                   = runtime.NewScheme()
	setupLog                 = ctrl.Log.WithName("setup")
	defaultNonTlsMetricsAddr = ":8083"
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = aoapis.AddToScheme(scheme)
	_ = operatorsv1.AddToScheme(scheme)
	_ = operatorsv1alpha1.AddToScheme(scheme)
	_ = configv1.AddToScheme(scheme)
	_ = monitoringv1.AddToScheme(scheme)
}

type options struct {
	metricsAddr                 string
	pprofAddr                   string
	enableLeaderElection        bool
	enableMetricsRecorder       bool
	enableMetricsTLSTermination bool
	probeAddr                   string
	metricsTlsCertDir           string
	nonTLSMetricsAddr           string
	metricsTlsConfig            tlsConfig
}

type tlsConfig struct {
	caCertPath string
	certPath   string
	keyPath    string
}

func parseFlags() *options {
	opts := &options{}

	flag.StringVar(&opts.metricsAddr, "metrics-addr", ":8080", "The address at which metrics (https or http) will be exposed.")
	flag.StringVar(&opts.pprofAddr, "pprof-addr", "", "The address the pprof web endpoint binds to.")
	flag.BoolVar(&opts.enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&opts.probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	flag.BoolVar(&opts.enableMetricsRecorder, "enable-metrics-recorder", true, "Enable recording Addon Metrics")
	flag.BoolVar(&opts.enableMetricsTLSTermination, "enable-metrics-tls-termination", true, "Enable metrics endpoint to be TLS-terminated")
	flag.StringVar(&opts.metricsTlsCertDir, "metrics-tls-cert-dir", "/tmp/k8s-metrics-server/serving-certs/", "Path to the directory where the TLS config-related files exist (ca.crt, tls.crt, tls.key)")
	flag.Parse()

	return opts
}

func preprocessOpts(opts *options) {
	// opts.nonTLSMetricsAddr - where the actual controller-runtime metric server will be setup
	// opts.metricsAddr - where the relay server will be setup if --enable-metrics-tls-termination flag would be provided
	// For the end-user/client, if --enable-metrics-tls-termination is provided, https://<host>:<metrics-addr>/metrics will be reachable
	// else, http://<host>:<metrics-addr>/metrics will be reachable
	if opts.enableMetricsTLSTermination {
		opts.nonTLSMetricsAddr = defaultNonTlsMetricsAddr
	} else {
		opts.nonTLSMetricsAddr = opts.metricsAddr
	}

	// add a trailing slash to the metrics TLS cert-dir path, if it doesn't exist
	if string(opts.metricsTlsCertDir[len(opts.metricsTlsCertDir)-1]) != "/" {
		opts.metricsTlsCertDir += "/"
	}

	// setup metrics tls config w.r.t metricsTlsCertDir
	opts.metricsTlsConfig = tlsConfig{
		caCertPath: opts.metricsTlsCertDir + "ca.crt",
		certPath:   opts.metricsTlsCertDir + "tls.crt",
		keyPath:    opts.metricsTlsCertDir + "tls.key",
	}
}

func initReconcilers(mgr ctrl.Manager, recorder *metrics.Recorder) {
	// Create a client that does not cache resources cluster-wide.
	uncachedClient, err := client.New(
		mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme(), Mapper: mgr.GetRESTMapper()})
	if err != nil {
		setupLog.Error(err, "unable to set up uncached client")
		os.Exit(1)
	}

	addonReconciler := &addoncontroller.AddonReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("Addon"),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
	}

	if err := addonReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Addon")
		os.Exit(1)
	}

	if err := (&aocontroller.AddonOperatorReconciler{
		Client:             mgr.GetClient(),
		UncachedClient:     uncachedClient,
		Log:                ctrl.Log.WithName("controllers").WithName("AddonOperator"),
		Scheme:             mgr.GetScheme(),
		GlobalPauseManager: addonReconciler,
		OCMClientManager:   addonReconciler,
		Recorder:           recorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AddonOperator")
		os.Exit(1)
	}

	if err := (&aicontroller.AddonInstanceReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controller").WithName("AddonInstance"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AddonInstance")
		os.Exit(1)
	}
}

func initPprof(mgr ctrl.Manager, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s := &http.Server{Addr: addr, Handler: mux}
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
		setupLog.Error(err, "unable to create pprof server")
		os.Exit(1)
	}
}

func initMetricsRelayServer(mgr ctrl.Manager, httpsRelayAddr string, target string, tlsConf tlsConfig) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		targetAddr := target
		if string(targetAddr[0]) == ":" {
			targetAddr = "http://127.0.0.1" + targetAddr
		}
		targetAddr += r.URL.Path

		resp, err := http.Get(targetAddr) //nolint
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("{\"code: %d\", \"message\": \"failed to call %s\", \"error\": \"%+v\"}", http.StatusInternalServerError, targetAddr, err))) //nolint
			return
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("{\"code: %d\", \"message\": \"failed to parse the response body received from GET %s\", \"error\": \"%+v\"}", http.StatusInternalServerError, targetAddr, err))) //nolint
			return
		}

		w.WriteHeader(resp.StatusCode)
		w.Write(bodyBytes) //nolint
	})

	s := &http.Server{Addr: httpsRelayAddr, Handler: mux}

	err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		errCh := make(chan error)
		defer func() {
			for range errCh {
			} // drain errCh for GC
		}()
		go func() {
			defer close(errCh)
			errCh <- s.ListenAndServeTLS(tlsConf.certPath, tlsConf.keyPath)
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
		setupLog.Error(err, "unable to create metrics relay server")
		os.Exit(1)
	}
}

func main() {
	opts := parseFlags()
	preprocessOpts(opts)

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         opts.nonTLSMetricsAddr,
		HealthProbeBindAddress:     opts.probeAddr,
		Port:                       9443,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.enableLeaderElection,
		LeaderElectionID:           "8a4hp84a6s.addon-operator-lock",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// PPROF
	if len(opts.pprofAddr) > 0 {
		initPprof(mgr, opts.pprofAddr)
	}

	if opts.enableMetricsTLSTermination {
		initMetricsRelayServer(mgr, opts.metricsAddr, opts.nonTLSMetricsAddr, opts.metricsTlsConfig)
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	var recorder *metrics.Recorder
	if opts.enableMetricsRecorder {
		recorder = metrics.NewRecorder(true)
	}
	initReconcilers(mgr, recorder)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
