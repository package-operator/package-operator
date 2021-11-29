package addon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	internalhandler "github.com/openshift/addon-operator/internal/controllers/addon/handler"
	"github.com/openshift/addon-operator/internal/ocm"
)

// Default timeout when we do a manual RequeueAfter
const (
	defaultRetryAfterTime = 10 * time.Second
	cacheFinalizer        = "addons.managed.openshift.io/cache"
)

type AddonReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	csvEventHandler csvEventHandler
	globalPause     bool
	globalPauseMux  sync.RWMutex
	addonRequeueCh  chan event.GenericEvent

	ocmClient    ocmClient
	ocmClientMux sync.RWMutex
}

type ocmClient interface {
	PatchUpgradePolicy(
		ctx context.Context,
		req ocm.UpgradePolicyPatchRequest,
	) (res ocm.UpgradePolicyPatchResponse, err error)
}

func (r *AddonReconciler) InjectOCMClient(ctx context.Context, c *ocm.Client) error {
	r.ocmClientMux.Lock()
	defer r.ocmClientMux.Unlock()

	if r.ocmClient == nil {
		r.Log.Info("ocm client initialized for the first time")

		// Requeue all addons for the first time that the ocm client becomes available.
		if err := r.requeueAllAddons(ctx); err != nil {
			return fmt.Errorf("requeue all Addons: %w", err)
		}
	}

	r.ocmClient = c
	return nil
}

// Pauses reconcilation of all Addon objects. Concurrency safe.
func (r *AddonReconciler) EnableGlobalPause(ctx context.Context) error {
	return r.setGlobalPause(ctx, true)
}

// Unpauses reconcilation of all Addon objects. Concurrency safe.
func (r *AddonReconciler) DisableGlobalPause(ctx context.Context) error {
	return r.setGlobalPause(ctx, false)
}

func (r *AddonReconciler) setGlobalPause(ctx context.Context, paused bool) error {
	r.globalPauseMux.Lock()
	defer r.globalPauseMux.Unlock()
	r.globalPause = paused

	if err := r.requeueAllAddons(ctx); err != nil {
		return fmt.Errorf("requeue all Addons: %w", err)
	}
	return nil
}

// requeue all addons that are currently in the local cache.
func (r *AddonReconciler) requeueAllAddons(ctx context.Context) error {
	addonList := &addonsv1alpha1.AddonList{}
	if err := r.List(ctx, addonList); err != nil {
		return fmt.Errorf("listing Addons, %w", err)
	}
	for i := range addonList.Items {
		r.addonRequeueCh <- event.GenericEvent{Object: &addonList.Items[i]}
	}
	return nil
}

type csvEventHandler interface {
	handler.EventHandler
	Free(addon *addonsv1alpha1.Addon)
	ReplaceMap(addon *addonsv1alpha1.Addon, csvKeys ...client.ObjectKey) (changed bool)
}

func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.csvEventHandler = internalhandler.NewCSVEventHandler()
	r.addonRequeueCh = make(chan event.GenericEvent)
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.Addon{}).
		Owns(&corev1.Namespace{}).
		Owns(&operatorsv1.OperatorGroup{}).
		Owns(&operatorsv1alpha1.CatalogSource{}).
		Owns(&operatorsv1alpha1.Subscription{}).
		Owns(&addonsv1alpha1.AddonInstance{}).
		Watches(&source.Kind{
			Type: &operatorsv1alpha1.ClusterServiceVersion{},
		}, r.csvEventHandler).
		Watches(&source.Channel{ // Requeue everything when entering/leaving global pause.
			Source: r.addonRequeueCh,
		}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

// AddonReconciler/Controller entrypoint
func (r *AddonReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	log := r.Log.WithValues("addon", req.NamespacedName.String())

	addon := &addonsv1alpha1.Addon{}
	if err := r.Get(ctx, req.NamespacedName, addon); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Ensure we report to the UpgradePolicy endpoint, when we are done with whatever we are doing.
	defer func() {
		if err != nil {
			return
		}

		err = r.handleUpgradePolicyStatusReporting(ctx, log, addon)
	}()

	// check for global pause
	r.globalPauseMux.RLock()
	defer r.globalPauseMux.RUnlock()
	if r.globalPause {
		err = r.reportAddonPauseStatus(ctx, addonsv1alpha1.AddonOperatorReasonPaused,
			addon)
		if err != nil {
			return ctrl.Result{}, err
		}
		// TODO: figure out how we can continue to report status
		return ctrl.Result{}, nil
	}

	// check for Addon pause
	if addon.Spec.Paused {
		err = r.reportAddonPauseStatus(ctx, addonsv1alpha1.AddonReasonPaused,
			addon)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Make sure Pause condition is removed
	if err := r.removeAddonPauseCondition(ctx, addon); err != nil {
		return ctrl.Result{}, err
	}

	if !addon.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.handleAddonDeletion(ctx, addon)
	}

	// Phase 0.
	// Ensure cache finalizer
	if !controllerutil.ContainsFinalizer(addon, cacheFinalizer) {
		controllerutil.AddFinalizer(addon, cacheFinalizer)
		if err := r.Update(ctx, addon); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	// Phase 1.
	// Ensure wanted namespaces
	if stopAndRetry, err := r.ensureWantedNamespaces(ctx, addon); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure wanted Namespaces: %w", err)
	} else if stopAndRetry {
		return ctrl.Result{
			RequeueAfter: defaultRetryAfterTime,
		}, nil
	}

	// Phase 2.
	// Ensure unwanted namespaces are removed
	if err := r.ensureDeletionOfUnwantedNamespaces(ctx, addon); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure deletion of unwanted Namespaces: %w", err)
	}

	// Phase 3.
	// Ensure the creation of the corresponding AddonInstance in .spec.install.olmOwnNamespace/.spec.install.olmAllNamespaces namespace
	if err := r.ensureAddonInstance(ctx, log, addon); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure the creation of addoninstance: %w", err)
	}

	// Phase 4.
	// Ensure OperatorGroup
	if stop, err := r.ensureOperatorGroup(ctx, log, addon); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure OperatorGroup: %w", err)
	} else if stop {
		return ctrl.Result{}, nil
	}

	// Phase 5.
	ensureResult, catalogSource, err := r.ensureCatalogSource(ctx, log, addon)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure CatalogSource: %w", err)
	}
	switch ensureResult {
	case ensureCatalogSourceResultRetry:
		log.Info("requeuing", "reason", "catalogsource unready")
		return ctrl.Result{
			RequeueAfter: defaultRetryAfterTime,
		}, nil
	case ensureCatalogSourceResultStop:
		return ctrl.Result{}, nil
	}

	// Phase 6.
	// Ensure Subscription for this Addon.
	currentCSVKey, requeue, err := r.ensureSubscription(
		ctx, log.WithName("phase-ensure-subscription"),
		addon, catalogSource)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure Subscription: %w", err)
	} else if requeue {
		return ctrl.Result{
			RequeueAfter: defaultRetryAfterTime,
		}, nil
	}

	// Phase 7.
	// Observe current csv
	if requeue, err := r.observeCurrentCSV(ctx, addon, currentCSVKey); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to observe current CSV: %w", err)
	} else if requeue {
		log.Info("requeuing", "reason", "csv unready")
		return ctrl.Result{
			RequeueAfter: defaultRetryAfterTime,
		}, nil
	}

	// After last phase and if everything is healthy
	if err = r.reportReadinessStatus(ctx, addon); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to report readiness status: %w", err)
	}

	return ctrl.Result{}, nil
}
