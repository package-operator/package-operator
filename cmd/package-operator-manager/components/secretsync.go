package components

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"package-operator.run/internal/controllers/secretsync"
	"package-operator.run/internal/dynamiccache"
)

type SecretSyncController struct{ controller }

func ProvideSecretSyncController(
	mgr ctrl.Manager, log logr.Logger,
	dc *dynamiccache.Cache,
) SecretSyncController {
	return SecretSyncController{
		secretsync.NewController(
			mgr.GetClient(),
			log.WithName("controllers").WithName("SecretSync"),
			mgr.GetScheme(),
			dc,
		),
	}
}
