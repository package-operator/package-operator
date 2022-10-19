package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkoapis "package-operator.run/apis"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = pkoapis.AddToScheme(scheme)
}

func main() {
	opts := loaderOpts{}
	flag.StringVar(&opts.packageName, "package-name", "", "Name of the package")
	flag.StringVar(&opts.packageNamespace, "package-namespace", "", "Target namespace associated with the package")
	flag.StringVar(&opts.packageDir, "package-dir", "", "Path to the directory containing the package's bundles")
	flag.Var(&opts.scope, "scope", "Scope of the ObjectDeployment to be created. 'cluster' creates ClusterObjectDeployment, 'namespace' creates ObjectDeployment")
	flag.BoolVar(&opts.ensureNamespace, "ensure-namespace", false, "Create packageNamespace if it doesn't exist. Only works with 'namespace' scope")
	flag.Var(&opts.labels, "labels", "Comma-separated list of labels to be propagated to the ObjectDeployment")
	flag.BoolVar(&opts.debugMode, "debug", false, "Toggle debug mode for package-loader")
	flag.Parse()

	if err := opts.isValid(); err != nil {
		log.Fatal("arguments found to be invalid: ", err.Error())
	}

	if opts.debugMode {
		log.SetLevel(log.DebugLevel)
	}

	log.Debug("parsed options", opts)

	// not worthwhile to create a cache-backed client as it will be anyway used for only one operation (create or update the objectDeployment) in its lifetime
	uncachedClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		log.Fatal("failed to set up uncached client", err)
	}
	log.Debug("new uncached client created successfully from the accessible config")

	if opts.ensureNamespace && opts.scope == namespaceScope {
		if err := ensureNamespace(uncachedClient, opts.packageNamespace); err != nil {
			log.Fatalf("failed to ensure namespace '%s': %s", opts.packageNamespace, err.Error())
		}
		log.Debugf("successfully ensured the existence of the namespace: '%s'", opts.packageNamespace)
	}

	phases, probes, err := processPhasesAndProbesFromPackageDir(opts.packageDir)
	if err != nil {
		log.Fatalf("failed to process Phases and Probes from the package-dir '%s': %s", opts.packageDir, err.Error())
	}
	log.Debugf("processed Phases and Probes successfully from the package dir '%s'", opts.packageDir)

	if err := deployPackage(uncachedClient, string(opts.scope), opts.packageName, opts.packageNamespace, phases, probes, opts.labels); err != nil {
		log.Fatal(err)
	}
	log.Debug("deployed the ObjectDeployment/ClusterObjectDeployment successfully")
}
