package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Default timeout when we do a manual RequeueAfter
const defaultRetryAfterTime = 10 * time.Second

type AddonReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.Addon{}).
		Owns(&corev1.Namespace{}).
		Complete(r)
}

// AddonReconciler/Controller entrypoint
func (r *AddonReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	addon := &addonsv1alpha1.Addon{}
	err := r.Get(ctx, req.NamespacedName, addon)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !addon.DeletionTimestamp.IsZero() {
		// Addon was already deleted and we don't need to do any cleanup for now
		// since kubernetes will garbage collect our child objects

		if addon.Status.Phase == addonsv1alpha1.Terminating {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, r.reportTerminationSignals(ctx, addon)
	}

	// Phase 1.
	// Ensure wanted namespaces
	r.Log.Info("Ensuring wanted Namespaces for Addon", "name", req.Name)
	stopAndRetry, err := r.ensureWantedNamespaces(ctx, addon)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure wanted namespaces: %w", err)
	}
	if stopAndRetry {
		return ctrl.Result{
			RequeueAfter: defaultRetryAfterTime,
		}, nil
	}

	// Phase 2.
	// Ensure unwanted namespaces are removed
	r.Log.Info("Ensuring deletion of unwanted Namespaces for Addon", "name", req.Name)
	err = r.ensureDeletionOfUnwantedNamespaces(ctx, addon)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure deletion of unwanted Namespaces: %w", err)
	}

	// After last phase and if everything is healthy
	r.Log.Info("Successfully reconciled Addon", "name", req.Name)
	return ctrl.Result{}, r.reportReadinessSignals(ctx, addon)
}

// Report Addon status to communicate that everything is alright
func (r *AddonReconciler) reportReadinessSignals(
	ctx context.Context, addon *addonsv1alpha1.Addon) error {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Available,
		Status:             metav1.ConditionTrue,
		Reason:             "FullyReconciled",
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.Ready
	return r.Status().Update(ctx, addon)
}

// Report Addon status to communicate that the Addon is terminating
func (r *AddonReconciler) reportTerminationSignals(
	ctx context.Context, addon *addonsv1alpha1.Addon) error {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Available,
		Status:             metav1.ConditionFalse,
		Reason:             "Terminating",
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.Terminating
	return r.Status().Update(ctx, addon)
}
