package controllers

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

func (r *AddonOperatorReconciler) getAllAddons(ctx context.Context) ([]addonsv1alpha1.Addon, error) {
	addonList := addonsv1alpha1.AddonList{}
	err := r.List(ctx, &addonList)
	if err != nil {
		return []addonsv1alpha1.Addon{}, err
	}
	return addonList.Items, nil
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
		r.GlobalPauseManager.SetGlobalPause(true) // pause

		// Get all Addons
		addons, err := r.getAllAddons(ctx)
		if err != nil {
			return err
		}

		// Update condition on all Addons
		for _, addon := range addons {
			err := reportAddonPauseStatus(ctx, addonsv1alpha1.AddonOperatorReasonPaused,
				r.Client, &addon)
			if err != nil {
				return err
			}
		}

		// Finally report Pause condition back to AddonOperator
		err = r.reportAddonOperatorPauseStatus(ctx, addonOperator)
		if err != nil {
			return err
		}
		return nil
	}

	// Unpause only if the current reported condition is Paused
	if !meta.IsStatusConditionTrue(addonOperator.Status.Conditions,
		addonsv1alpha1.Paused) {
		return nil
	}
	r.GlobalPauseManager.SetGlobalPause(false) // unpause

	// Get all Addons
	addons, err := r.getAllAddons(ctx)
	if err != nil {
		return err
	}

	for _, addon := range addons {
		err := removeAddonPauseCondition(ctx, r.Client, &addon)
		if err != nil {
			return err
		}
	}

	// Finally remove Paused condition from AddonOperator
	err = r.removeAddonOperatorPauseCondition(ctx, addonOperator)
	if err != nil {
		return err
	}
	return nil
}
