package controllers

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openshift/addon-operator/apis"
	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	internalhandler "github.com/openshift/addon-operator/internal/handler"
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
}

func (r *AddonReconciler) SetGlobalPause(paused bool) {
	r.globalPauseMux.Lock()
	defer r.globalPauseMux.Unlock()
	r.globalPause = paused
}

type csvEventHandler interface {
	handler.EventHandler
	Free(addon *addonsv1alpha1.Addon)
	ReplaceMap(addon *addonsv1alpha1.Addon, csvKeys ...client.ObjectKey) (changed bool)
}

func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.csvEventHandler = internalhandler.NewCSVEventHandler()

	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.Addon{}).
		Owns(&corev1.Namespace{}).
		Owns(&operatorsv1.OperatorGroup{}).
		Owns(&operatorsv1alpha1.CatalogSource{}).
		Owns(&operatorsv1alpha1.Subscription{}).
		Watches(&source.Kind{
			Type: &operatorsv1alpha1.ClusterServiceVersion{},
		}, r.csvEventHandler).
		Complete(r)
}

// AddonReconciler/Controller entrypoint
func (r *AddonReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("addon", req.NamespacedName.String())

	addon := &addonsv1alpha1.Addon{}
	err := r.Get(ctx, req.NamespacedName, addon)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check for global pause.
	// addon status reporting handled by AddonOperator reconciler
	r.globalPauseMux.RLock()
	defer r.globalPauseMux.RUnlock()
	if r.globalPause {
		// TODO: figure out how we can continue to report status
		return ctrl.Result{}, nil
	}

	// check for Addon pause
	if addon.Spec.Paused {
		err = reportAddonPauseStatus(ctx, apis.AddonReasonPaused,
			r.Client, addon)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Make sure Pause condition is removed
	if err := removeAddonPauseCondition(ctx, r.Client, addon); err != nil {
		return ctrl.Result{}, nil
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
	// Ensure OperatorGroup
	if stop, err := r.ensureOperatorGroup(ctx, log, addon); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure OperatorGroup: %w", err)
	} else if stop {
		return ctrl.Result{}, nil
	}

	// Phase 4.
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

	// Phase 5.
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

	// Phase 6.
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
