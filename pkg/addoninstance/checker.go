package addoninstance

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func RunHeartbeatChecker(ctx context.Context, log logr.Logger, mgr manager.Manager, rate time.Duration) {
	// TODO: implement circuit breaker here
	// run the checker at an interval of `rate`
	for range time.Tick(rate) {
		addonInstances := &addonsv1alpha1.AddonInstanceList{}
		if err := mgr.GetCache().List(ctx, addonInstances); err != nil {
			log.Error(fmt.Errorf("failed to fetch the AddonInstanceList: %w", err), "retrying heartbeat check")
			continue
		}
		for _, addonInstance := range addonInstances.Items {
			addonInstance := addonInstance

			now, lastHeartbeatTime := metav1.Now(), addonInstance.Status.LastHeartbeatTime
			if lastHeartbeatTime.IsZero() {
				log.Info(fmt.Sprintf("skipping heartbeat for %s/%s because no lastHeartbeatTime was found.", addonInstance.Namespace, addonInstance.Name))
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
					meta.SetStatusCondition(&addonInstance.Status.Conditions, heartbeatTimeoutCondition)
					if err := mgr.GetClient().Status().Update(ctx, &addonInstance); err != nil {
						log.Error(fmt.Errorf("failed to fetch the AddonInstanceList: %w", err), "retrying heartbeat check")
						continue
					}
				}
			}
		}
	}
}
