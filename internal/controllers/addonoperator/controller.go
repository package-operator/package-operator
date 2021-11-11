package addonoperator

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	defaultAddonOperatorRequeueTime = time.Minute
)

type AddonOperatorReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	GlobalPauseManager globalPauseManager
}

func (r *AddonOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.AddonOperator{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Watches(source.Func(enqueueAddonOperator),
			&handler.EnqueueRequestForObject{}). // initial enqueue for creating the object
		Complete(r)
}

func enqueueAddonOperator(ctx context.Context, h handler.EventHandler,
	q workqueue.RateLimitingInterface, p ...predicate.Predicate) error {
	q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Name: addonsv1alpha1.DefaultAddonOperatorName,
	}})
	return nil
}

func (r *AddonOperatorReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := r.Log.WithValues("addon-operator", req.NamespacedName.String())

	addonOperator := &addonsv1alpha1.AddonOperator{}
	err := r.Get(ctx, client.ObjectKey{
		Name: addonsv1alpha1.DefaultAddonOperatorName,
	}, addonOperator)
	// Create default AddonOperator object if it doesn't exist
	if apierrors.IsNotFound(err) {
		log.Info("default AddonOperator not found")
		return ctrl.Result{}, r.handleAddonOperatorCreation(ctx, log)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.handleGlobalPause(ctx, addonOperator); err != nil {
		return ctrl.Result{}, fmt.Errorf("handling global pause: %w", err)
	}

	// TODO: This is where all the checking / validation happens
	// for "in-depth" status reporting

	err = r.reportAddonOperatorReadinessStatus(ctx, addonOperator)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: defaultAddonOperatorRequeueTime}, nil
}

func (r *AddonOperatorReconciler) handleGlobalPause(
	ctx context.Context, addonOperator *addonsv1alpha1.AddonOperator) error {
	// Check if addonoperator.spec.paused == true
	if addonOperator.Spec.Paused {
		// Check if Paused condition has already been reported
		if meta.IsStatusConditionTrue(addonOperator.Status.Conditions,
			addonsv1alpha1.Paused) {
			return nil
		}
		if err := r.GlobalPauseManager.EnableGlobalPause(ctx); err != nil {
			return fmt.Errorf("setting global pause: %w", err)
		}
		if err := r.reportAddonOperatorPauseStatus(ctx, addonOperator); err != nil {
			return fmt.Errorf("report AddonOperator paused: %w", err)
		}
		return nil
	}

	// Unpause only if the current reported condition is Paused
	if !meta.IsStatusConditionTrue(addonOperator.Status.Conditions,
		addonsv1alpha1.Paused) {
		return nil
	}
	if err := r.GlobalPauseManager.DisableGlobalPause(ctx); err != nil {
		return fmt.Errorf("removing global pause: %w", err)
	}
	if err := r.removeAddonOperatorPauseCondition(ctx, addonOperator); err != nil {
		return fmt.Errorf("remove AddonOperator paused: %w", err)
	}
	return nil
}
