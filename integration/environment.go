package integration

import (
	"context"
	"fmt"

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
	Config *rest.Config
	Scheme = runtime.NewScheme()

	// Namespace that the Package Operator is running in.
	// Needs to be auto-discovered, because OpenShift CI is installing the Operator in a non deterministic namespace.
	PackageOperatorNamespace string
)

func init() {
	ctx := context.Background()
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

	Config = ctrl.GetConfigOrDie()

	var err error
	Client, err = client.New(Config, client.Options{
		Scheme: Scheme,
	})
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
		panic(fmt.Errorf("no packageOperator deployment found on the cluster!"))
	case 1:
		packageOperatorNamespace = packageOperatorDeployments[0].Namespace
	default:
		panic(fmt.Errorf("multiple packageOperator deployments found on the cluster!"))
	}
	return
}
