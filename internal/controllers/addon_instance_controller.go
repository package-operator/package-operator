package controllers

// commenting this as currently, it's not serving any purpose but might end up with some use case later on where, say, dedicated reconciliation of AddonInstance is resource ends up being required
// import (
// 	"context"

// 	"github.com/go-logr/logr"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	ctrl "sigs.k8s.io/controller-runtime"
// 	"sigs.k8s.io/controller-runtime/pkg/client"

// 	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
// )

// type AddonInstanceReconciler struct {
// 	client.Client
// 	Log    logr.Logger
// 	Scheme *runtime.Scheme
// }

// func (r *AddonInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
// 	return ctrl.NewControllerManagedBy(mgr).
// 		For(&addonsv1alpha1.AddonInstance{}).
// 		Complete(r)
// }

// // AddonInstanceReconciler/Controller entrypoint
// func (r *AddonInstanceReconciler) Reconcile(
// 	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
// 	_ = r.Log.WithValues("addon", req.NamespacedName.String())
// 	return ctrl.Result{}, nil
// }
