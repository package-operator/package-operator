package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"package-operator.run/internal/packages"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
)

// GenericRepositoryController reconciles both Repository and ClusterRepository objects.
type GenericRepositoryController struct {
	newRepository adapters.GenericRepositoryFactory

	client              client.Client
	log                 logr.Logger
	scheme              *runtime.Scheme
	retriever           RepoRetriever
	store               packages.RepositoryStore
	backoff             *flowcontrol.Backoff
	packageHashModifier *int32
}

func NewRepositoryController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme, store packages.RepositoryStore,
) *GenericRepositoryController {
	return newGenericRepositoryController(adapters.NewGenericRepository,
		c, log, scheme, &CraneRepoRetriever{}, store)
}

func NewClusterRepositoryController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme, store packages.RepositoryStore,
) *GenericRepositoryController {
	return newGenericRepositoryController(adapters.NewGenericClusterRepository,
		c, log, scheme, &CraneRepoRetriever{}, store)
}

func newGenericRepositoryController(
	newRepository adapters.GenericRepositoryFactory,
	client client.Client, log logr.Logger,
	scheme *runtime.Scheme,
	retriever RepoRetriever, store packages.RepositoryStore,
) *GenericRepositoryController {
	var boCfg controllers.BackoffConfig
	boCfg.Default()

	controller := &GenericRepositoryController{
		newRepository: newRepository,
		client:        client,
		log:           log,
		scheme:        scheme,
		retriever:     retriever,
		store:         store,
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

func (c *GenericRepositoryController) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	log := c.log.WithValues("Repository", req.String())
	defer log.Info("reconciled")

	ctx = logr.NewContext(ctx, log)

	nsName := req.NamespacedName

	repo := c.newRepository(c.scheme)
	err = c.client.Get(ctx, nsName, repo.ClientObject())
	switch {
	// repo was somehow deleted
	case apierrors.IsNotFound(err):
		if c.store.Contains(nsName) {
			c.store.Delete(nsName)
		}
		return res, nil

	case err != nil:
		return res, err
	}

	defer c.backoff.GC()

	specHash := repo.GetSpecHash(c.packageHashModifier)
	if c.store.Contains(nsName) && repo.GetUnpackedHash() == specHash {
		// We have already unpacked this repository \o/
		return res, nil
	}

	idx, err := c.retriever.Retrieve(ctx, repo.GetImage())
	switch {
	case errors.Is(err, ErrRepoRetrieverPull):
		c.setStatusCondition(repo,
			corev1alpha1.PackageUnpacked, metav1.ConditionFalse,
			"ImagePullBackOff", err.Error())
		res = ctrl.Result{RequeueAfter: c.nextBackoff(string(repo.ClientObject().GetUID()))}

	case errors.Is(err, ErrRepoRetrieverLoad):
		c.setStatusCondition(repo,
			corev1alpha1.PackageInvalid, metav1.ConditionTrue,
			"Invalid", err.Error())

	default:
		c.store.Store(idx, nsName)
		repo.SetUnpackedHash(specHash)
		c.setStatusCondition(repo,
			corev1alpha1.PackageUnpacked, metav1.ConditionTrue,
			"UnpackSuccess", "Unpack job succeeded")
		c.setStatusCondition(repo,
			corev1alpha1.PackageAvailable, metav1.ConditionTrue,
			"Available", "Latest Revision is Available.")
	}

	return res, c.updateStatus(ctx, repo)
}

func (c *GenericRepositoryController) setStatusCondition(repo adapters.GenericRepositoryAccessor,
	typ string, status metav1.ConditionStatus, reason string, message string,
) {
	meta.SetStatusCondition(
		repo.GetConditions(), metav1.Condition{
			Type:               typ,
			Status:             status,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: repo.ClientObject().GetGeneration(),
		})
}

func (c *GenericRepositoryController) nextBackoff(backoffID string) time.Duration {
	c.backoff.Next(backoffID, c.backoff.Clock.Now())
	return c.backoff.Get(backoffID)
}

func (c *GenericRepositoryController) updateStatus(ctx context.Context, repo adapters.GenericRepositoryAccessor) error {
	repo.UpdatePhase()
	if err := c.client.Status().Update(ctx, repo.ClientObject()); err != nil {
		return fmt.Errorf("updating Package status: %w", err)
	}
	return nil
}
