package addoninstance

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
)

type AddonInstanceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	heartbeatCheckerRate time.Duration
}

func (r *AddonInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.heartbeatCheckerRate = controllers.DefaultAddonInstanceHeartbeatUpdatePeriod.Duration
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.AddonInstance{}).
		Complete(r)
}

// AddonInstanceReconciler/Controller entrypoint
func (r *AddonInstanceReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("addoninstance", req.NamespacedName.String())

	addonInstance := &addonsv1alpha1.AddonInstance{}
	if err := r.Get(ctx, req.NamespacedName, addonInstance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now, lastHeartbeatTime := metav1.Now(), addonInstance.Status.LastHeartbeatTime
	if lastHeartbeatTime.IsZero() {
		return ctrl.Result{RequeueAfter: r.heartbeatCheckerRate}, nil
	}

	diff := int64(now.Time.Sub(lastHeartbeatTime.Time) / time.Second)
	threshold := controllers.DefaultAddonInstanceHeartbeatTimeoutThresholdMultiplier *
		int64(addonInstance.Spec.HeartbeatUpdatePeriod.Duration/time.Second)

	// if the last heartbeat is older than the timeout threshold,
	// register HeartbeatTimeout Condition
	if diff >= threshold {
		// check if already HeartbeatTimeout condition exists
		// if it does, no need to update it
		existingCondition := meta.FindStatusCondition(addonInstance.Status.Conditions,
			addonsv1alpha1.AddonInstanceHealthy)
		if existingCondition == nil || existingCondition.Reason != "HeartbeatTimeout" {
			log.Info(fmt.Sprintf("setting the Condition of %s/%s to 'HeartbeatTimeout'",
				addonInstance.Namespace, addonInstance.Name))
			// change the following line to:
			// meta.SetStatusCondition(&addonInstance.Status.Conditions,
			// addoninstanceapi.HeartbeatTimeoutCondition),
			// once https://github.com/openshift/addon-operator/pull/91 gets merged
			meta.SetStatusCondition(&addonInstance.Status.Conditions, metav1.Condition{
				Type:    "addons.managed.openshift.io/Healthy",
				Status:  "Unknown",
				Reason:  "HeartbeatTimeout",
				Message: "Addon failed to send heartbeat.",
			})
			if err := r.Client.Status().Update(ctx, addonInstance); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{RequeueAfter: r.heartbeatCheckerRate}, nil
}
