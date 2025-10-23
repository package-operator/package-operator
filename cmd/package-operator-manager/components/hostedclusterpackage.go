package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/hostedclusterpackages"
)

func ProvideHostedClusterPackageController(
	mgr ctrl.Manager, log logr.Logger,
) hostedclusterpackages.HostedClusterPackageController {
	return *hostedclusterpackages.NewHostedClusterPackageController(
		mgr.GetClient(),
		log.WithName("controllers").WithName("HostedClusterPackage"),
		mgr.GetScheme(),
	)
}
