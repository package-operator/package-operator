package main

import (
	"flag"
	"os"

	aoapis "github.com/openshift/addon-operator/apis"
	"github.com/openshift/addon-operator/internal/webhooks"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = aoapis.AddToScheme(scheme)
}

func main() {
	var (
		port    int
		certDir string
	)

	flag.IntVar(&port, "port", 8080, "The port the webhook server binds to")
	flag.StringVar(&certDir, "cert-dir", "",
		"The directory that contains the server key and certificate")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		Port:               port,
		CertDir:            certDir,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("Setting up webhook server")

	// Register webhooks as handlers
	wbh := mgr.GetWebhookServer()
	wbh.Register("/validate-addon", &webhook.Admission{
		Handler: &webhooks.AddonWebhookHandler{
			Log:    log.Log.WithName("validating webhooks").WithName("Addon"),
			Client: mgr.GetClient(),
		},
	})

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
