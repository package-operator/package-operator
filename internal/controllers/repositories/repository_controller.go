package repositories

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
)

// GenericRepositoryController reconciles both Repository and ClusterRepository objects.
type GenericRepositoryController struct {
	newRepository adapters.GenericRepositoryFactory

	client  client.Client
	log     logr.Logger
	scheme  *runtime.Scheme
	backoff *flowcontrol.Backoff
}

func NewRepositoryController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericRepositoryController {
	return newGenericRepositoryController(adapters.NewGenericRepository,
		c, log, scheme)
}

func NewClusterRepositoryController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericRepositoryController {
	return newGenericRepositoryController(adapters.NewGenericClusterRepository,
		c, log, scheme)
}

func newGenericRepositoryController(
	newRepository adapters.GenericRepositoryFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
) *GenericRepositoryController {
	var boCfg controllers.BackoffConfig
	boCfg.Default()

	controller := &GenericRepositoryController{
		newRepository: newRepository,
		client:        client,
		log:           log,
		scheme:        scheme,
		backoff:       boCfg.GetBackoff(),
	}
	return controller
}

func (c *GenericRepositoryController) SetupWithManager(mgr ctrl.Manager) error {
	repo := c.newRepository(c.scheme).ClientObject()
	return ctrl.NewControllerManagedBy(mgr).
		For(repo).
		Complete(c)
}

func (c *GenericRepositoryController) Reconcile(_ context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	log := c.log.WithValues("Repository", req.String())
	defer log.Info("reconciled")

	// TODO: add logic here
	return res, nil
}
