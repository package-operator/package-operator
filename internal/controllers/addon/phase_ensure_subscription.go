package addon

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
)

func (r *AddonReconciler) ensureSubscription(
	ctx context.Context,
	log logr.Logger,
	addon *addonsv1alpha1.Addon,
	catalogSource *operatorsv1alpha1.CatalogSource,
) (
	requeueResult,
	client.ObjectKey,
	error,
) {
	var commonInstallOptions addonsv1alpha1.AddonInstallOLMCommon
	switch addon.Spec.Install.Type {
	case addonsv1alpha1.OLMAllNamespaces:
		commonInstallOptions = addon.Spec.Install.
			OLMAllNamespaces.AddonInstallOLMCommon
	case addonsv1alpha1.OLMOwnNamespace:
		commonInstallOptions = addon.Spec.Install.
			OLMOwnNamespace.AddonInstallOLMCommon
	}

	desiredSubscription := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.Name,
			Namespace: commonInstallOptions.Namespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          catalogSource.Name,
			CatalogSourceNamespace: catalogSource.Namespace,
			Channel:                commonInstallOptions.Channel,
			Package:                commonInstallOptions.PackageName,
			// InstallPlanApproval is deliberately unmanaged
			// API default is `Automatic`
			// Legacy behavior of existing managed-tenants tooling is:
			// All addons initially have to be installed with `Automatic`
			// so that the very first InstallPlan succeedes
			// but some addons want to take control of upgrades and thus
			// change the Subscription.Spec.InstallPlanApproval value to `Manual`
			// ATTENTION: When reconciling the subscription, we need to
			// make sure to keep the current value of this field
		},
	}
	controllers.AddCommonLabels(desiredSubscription.Labels, addon)
	if err := controllerutil.SetControllerReference(addon, desiredSubscription, r.Scheme); err != nil {
		return resultNil, client.ObjectKey{}, fmt.Errorf("setting controller reference: %w", err)
	}

	observedSubscription, err := r.reconcileSubscription(
		ctx, desiredSubscription)
	if err != nil {
		return resultNil, client.ObjectKey{}, fmt.Errorf("reconciling Subscription: %w", err)
	}

	if len(observedSubscription.Status.InstalledCSV) == 0 ||
		len(observedSubscription.Status.CurrentCSV) == 0 {
		log.Info("requeue", "reason", "csv not linked in subscription")
		return resultRetry, client.ObjectKey{}, nil
	}

	installedCSVKey := client.ObjectKey{
		Name:      observedSubscription.Status.InstalledCSV,
		Namespace: commonInstallOptions.Namespace,
	}
	currentCSVKey := client.ObjectKey{
		Name:      observedSubscription.Status.CurrentCSV,
		Namespace: commonInstallOptions.Namespace,
	}

	changed := r.csvEventHandler.ReplaceMap(addon, installedCSVKey, currentCSVKey)
	if changed {
		// Mapping changes need to requeue, because we could have lost events before or during
		// setting up the mapping, see csvEventHandler implementation for a longer description.
		log.Info("requeue", "reason", "csv-addon mapping changed")
		return resultRetry, client.ObjectKey{}, nil
	}

	return resultNil, currentCSVKey, nil
}

func (r *AddonReconciler) reconcileSubscription(
	ctx context.Context,
	subscription *operatorsv1alpha1.Subscription,
) (currentSubscription *operatorsv1alpha1.Subscription, err error) {
	currentSubscription = &operatorsv1alpha1.Subscription{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      subscription.Name,
		Namespace: subscription.Namespace,
	}, currentSubscription)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return subscription, r.Create(ctx, subscription)
		}
		return nil, err
	}

	// keep installPlanApproval value of existing object
	subscription.Spec.InstallPlanApproval = currentSubscription.Spec.InstallPlanApproval

	// only update when spec has changed or owner reference has changed
	if !equality.Semantic.DeepEqual(
		subscription.Spec, currentSubscription.Spec) ||
		!equality.Semantic.DeepEqual(
			subscription.OwnerReferences, currentSubscription.OwnerReferences) {
		// copy new spec into existing object and update in the k8s api
		currentSubscription.Spec = subscription.Spec
		currentSubscription.OwnerReferences = subscription.OwnerReferences
		return currentSubscription, r.Update(ctx, currentSubscription)
	}
	return currentSubscription, nil
}
