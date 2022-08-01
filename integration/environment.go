package integration

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// Client pointing to the e2e test cluster.
	Client client.Client
	// Config is the REST config used to connect to the cluster.
	Config *rest.Config
	// Scheme used by created clients.
	Scheme = runtime.NewScheme()

	// PackageOperatorNamespace is the namespace that the Package Operator is running in.
	// Needs to be auto-discovered, because OpenShift CI is installing the Operator in a non deterministic namespace.
	PackageOperatorNamespace string
)

func init() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := initClients(ctx); err != nil {
		panic(err)
	}

	var err error
	PackageOperatorNamespace, err = findPackageOperatorNamespace(ctx)
	if err != nil {
		panic(err)
	}
}

func initClients(ctx context.Context) error {
	// Client/Scheme setup.
	AddToSchemes := runtime.SchemeBuilder{
		clientgoscheme.AddToScheme,
	}
	if err := AddToSchemes.AddToScheme(Scheme); err != nil {
		return fmt.Errorf("could not load schemes: %w", err)
	}

	var err error

	Config, err = ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("get rest config: %w", err)
	}

	Client, err = client.New(Config, client.Options{Scheme: Scheme})
	if err != nil {
		return fmt.Errorf("creating runtime client: %w", err)
	}

	return nil
}

func findPackageOperatorNamespace(ctx context.Context) (
	packageOperatorNamespace string,
	err error,
) {
	// discover packageOperator Namespace
	deploymentList := &appsv1.DeploymentList{}
	// We can't use a label-selector, because OLM is overriding the deployment labels...
	if err := Client.List(ctx, deploymentList); err != nil {
		panic(fmt.Errorf("listing package-operator deployments on the cluster: %w", err))
	}
	var packageOperatorDeployments []appsv1.Deployment
	for _, deployment := range deploymentList.Items {
		if deployment.Name == "package-operator-manager" {
			packageOperatorDeployments = append(packageOperatorDeployments, deployment)
		}
	}
	switch len(packageOperatorDeployments) {
	case 0:
		panic("no packageOperator deployment found on the cluster")
	case 1:
		packageOperatorNamespace = packageOperatorDeployments[0].Namespace
	default:
		panic("multiple packageOperator deployments found on the cluster")
	}
	return
}
