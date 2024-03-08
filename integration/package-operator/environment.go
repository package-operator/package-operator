//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "package-operator.run/apis"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

const (
	defaultWaitTimeout  = 20 * time.Second
	defaultWaitInterval = 1 * time.Second
)

var (
	// Client pointing to the e2e test cluster.
	Client client.Client

	// DiscoveryClient pointing to the e2e test cluster.
	DiscoveryClient *discovery.DiscoveryClient

	// Config is the REST config used to connect to the cluster.
	Config *rest.Config
	// Scheme used by created clients.
	Scheme = runtime.NewScheme()

	Waiter *wait.Waiter

	// PackageOperatorNamespace is the namespace that the Package Operator is running in.
	// Needs to be auto-discovered, because OpenShift CI is installing the Operator in a non deterministic namespace.
	PackageOperatorNamespace string

	TestStubImage string
	// SuccessTestPackageImage points to an image to use to test Package installation.
	SuccessTestPackageImage string
	// SuccessTestMultiPackageImage points to an image to use to test multi-component Package installation.
	SuccessTestMultiPackageImage string
	// SuccessTestCelPackageImage points to an image to use to test Package installation with CEL annotations.
	SuccessTestCelPackageImage string

	FailureTestPackageImage = "localhost/does-not-exist"

	LatestSelfBootstrapJobURL string
)

func init() {
	SuccessTestPackageImage = os.Getenv("PKO_TEST_SUCCESS_PACKAGE_IMAGE")
	if len(SuccessTestPackageImage) == 0 {
		panic("PKO_TEST_SUCCESS_PACKAGE_IMAGE not set!")
	}
	SuccessTestMultiPackageImage = os.Getenv("PKO_TEST_SUCCESS_MULTI_PACKAGE_IMAGE")
	if len(SuccessTestMultiPackageImage) == 0 {
		panic("PKO_TEST_SUCCESS_MULTI_PACKAGE_IMAGE not set!")
	}
	SuccessTestCelPackageImage = os.Getenv("PKO_TEST_SUCCESS_CEL_PACKAGE_IMAGE")
	if len(SuccessTestMultiPackageImage) == 0 {
		panic("PKO_TEST_SUCCESS_CEL_PACKAGE_IMAGE not set!")
	}
	TestStubImage = os.Getenv("PKO_TEST_STUB_IMAGE")
	if len(TestStubImage) == 0 {
		panic("PKO_TEST_STUB_IMAGE not set!")
	}
	LatestSelfBootstrapJobURL = os.Getenv("PKO_TEST_LATEST_BOOTSTRAP_JOB")
	if len(LatestSelfBootstrapJobURL) == 0 {
		panic("PKO_TEST_LATEST_BOOTSTRAP_JOB not set!")
	}

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

	Waiter = wait.NewWaiter(Client, Scheme, wait.WithTimeout(defaultWaitTimeout), wait.WithInterval(defaultWaitInterval))
}

func initClients(_ context.Context) error {
	// Client/Scheme setup.
	AddToSchemes := runtime.SchemeBuilder{
		scheme.AddToScheme,
		apis.AddToScheme,
		hypershiftv1beta1.AddToScheme,
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

	DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(Config)
	if err != nil {
		return fmt.Errorf("creating discovery client: %w", err)
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
