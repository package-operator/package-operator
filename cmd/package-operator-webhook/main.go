package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"package-operator.run/package-operator/internal/webhooks"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = corev1alpha1.AddToScheme(scheme)
}

func main() {
	var (
		port      int
		certDir   string
		probeAddr string
	)

	flag.IntVar(&port, "port", 8080, "The port the webhook server binds to")
	flag.StringVar(&certDir, "cert-dir", "",
		"The directory that contains the server key and certificate")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     "0",
		Port:                   port,
		CertDir:                certDir,
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("Setting up webhook server")

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Register webhooks as handlers
	wbh := mgr.GetWebhookServer()
	wbh.Register("/validate-object-set", &webhook.Admission{
		Handler: webhooks.NewObjectSetWebhookHandler(
			log.Log.WithName("validating webhooks").WithName("ObjectSets"),
			mgr.GetClient(),
		),
	},
	)
	wbh.Register("/validate-object-set-phase", &webhook.Admission{
		Handler: webhooks.NewObjectSetPhaseWebhookHandler(
			log.Log.WithName("validating webhooks").WithName("ObjectSetPhases"),
			mgr.GetClient(),
		),
	},
	)
	wbh.Register("/validate-cluster-object-set", &webhook.Admission{
		Handler: webhooks.NewClusterObjectSetWebhookHandler(
			log.Log.WithName("validating webhooks").WithName("ClusterObjectSets"),
			mgr.GetClient(),
		),
	})
	wbh.Register("/validate-cluster-object-set-phase", &webhook.Admission{
		Handler: webhooks.NewClusterObjectSetPhaseWebhookHandler(
			log.Log.WithName("validating webhooks").WithName("ClusterObjectSetPhases"),
			mgr.GetClient(),
		),
	})

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
