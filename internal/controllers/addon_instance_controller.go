package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	addoninstanceapi "github.com/openshift/addon-operator/pkg/addoninstance"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AddonInstanceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	HeartbeatCheckerRate time.Duration
}

func (r *AddonInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonsv1alpha1.AddonInstance{}).
		Complete(r)
}

// AddonInstanceReconciler/Controller entrypoint
func (r *AddonInstanceReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("addoninstance", req.NamespacedName.String())

	addonInstance := &addonsv1alpha1.AddonInstance{}
	// r.Get() is backed by a Kubernetes Client backed by cached reads. Ref: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.10.2/pkg/cluster/cluster.go#L51-L55
	if err := r.Get(ctx, req.NamespacedName, addonInstance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	now, lastHeartbeatTime := metav1.Now(), addonInstance.Status.LastHeartbeatTime
	if lastHeartbeatTime.IsZero() {
		// is it worth logging this, considering the immense amounts of logs which are going to end up getting generated?
		//log.Info(fmt.Sprintf("skipping heartbeat for %s/%s because no lastHeartbeatTime was found.", addonInstance.Namespace, addonInstance.Name))
		return ctrl.Result{RequeueAfter: r.HeartbeatCheckerRate}, nil
	}

	diff := int64(now.Time.Sub(lastHeartbeatTime.Time) / time.Second)
	threshold := addonsv1alpha1.DefaultAddonInstanceHeartbeatTimeoutThresholdMultiplier * int64(addonInstance.Spec.HeartbeatUpdatePeriod.Duration/time.Second)

	// if the last heartbeat is older than the timeout threshold, register HeartbeatTimeout Condition
	if diff >= threshold {
		// check if already HeartbeatTimeout condition exists
		// if it does, no need to update it
		existingCondition := meta.FindStatusCondition(addonInstance.Status.Conditions, "addons.managed.openshift.io/Healthy")
		if existingCondition == nil || existingCondition.Reason != "HeartbeatTimeout" {
			log.Info(fmt.Sprintf("setting the Condition of %s/%s to 'HeartbeatTimeout'", addonInstance.Namespace, addonInstance.Name))
			meta.SetStatusCondition(&addonInstance.Status.Conditions, addoninstanceapi.HeartbeatTimeoutCondition)
			if err := r.Client.Status().Update(ctx, addonInstance); err != nil {
				log.Error(fmt.Errorf("failed to update the condition of the addoninstance: %w", err), "retrying heartbeat check")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{RequeueAfter: r.HeartbeatCheckerRate}, nil
}
