package addon

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
)

const catalogSourcePublisher = "OSD Red Hat Addons"

// Ensure existence of the CatalogSource specified in the given Addon resource
// returns an ensureCatalogSourceResult that signals the caller if they have to
// stop or retry reconciliation of the surrounding Addon resource
func (r *AddonReconciler) ensureCatalogSource(
	ctx context.Context, log logr.Logger, addon *addonsv1alpha1.Addon,
) (requeueResult, *operatorsv1alpha1.CatalogSource, error) {
	targetNamespace, catalogSourceImage, stop := r.parseAddonInstallConfig(log, addon)
	if stop {
		return resultStop, nil, nil
	}

	catalogSource := &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.Name,
			Namespace: targetNamespace,
		},
		Spec: operatorsv1alpha1.CatalogSourceSpec{
			SourceType:  operatorsv1alpha1.SourceTypeGrpc,
			Publisher:   catalogSourcePublisher,
			DisplayName: addon.Spec.DisplayName,
			Image:       catalogSourceImage,
		},
	}

	controllers.AddCommonLabels(catalogSource.Labels, addon)

	if err := controllerutil.SetControllerReference(addon, catalogSource, r.Scheme); err != nil {
		return resultNil, nil, err
	}

	var observedCatalogSource *operatorsv1alpha1.CatalogSource
	{
		var err error
		observedCatalogSource, err = reconcileCatalogSource(ctx, r.Client, catalogSource, addon.Spec.ResourceAdoptionStrategy)
		if err != nil {
			return resultNil, nil, err
		}
	}

	if observedCatalogSource.Status.GRPCConnectionState == nil {
		reportCatalogSourceUnreadinessStatus(addon, ".Status.GRPCConnectionState is nil")
		return resultRetry, nil, nil
	}
	if observedCatalogSource.Status.GRPCConnectionState.LastObservedState != "READY" {
		reportCatalogSourceUnreadinessStatus(
			addon,
			fmt.Sprintf(
				".Status.GRPCConnectionState.LastObservedState == %s",
				observedCatalogSource.Status.GRPCConnectionState.LastObservedState,
			),
		)
		return resultRetry, nil, nil
	}

	return resultNil, observedCatalogSource, nil
}

// reconciles a CatalogSource and returns a new CatalogSource object with observed state.
// Warning: Will adopt existing CatalogSource
func reconcileCatalogSource(ctx context.Context, c client.Client, catalogSource *operatorsv1alpha1.CatalogSource, strategy addonsv1alpha1.ResourceAdoptionStrategyType) (
	*operatorsv1alpha1.CatalogSource, error) {
	currentCatalogSource := &operatorsv1alpha1.CatalogSource{}

	{
		err := c.Get(ctx, client.ObjectKey{
			Name:      catalogSource.Name,
			Namespace: catalogSource.Namespace,
		}, currentCatalogSource)
		if err != nil {
			if k8sApiErrors.IsNotFound(err) {
				return catalogSource, c.Create(ctx, catalogSource)
			}
			return nil, err
		}
	}

	// only update when spec or ownerReference has changed
	ownedByAddon := controllers.HasEqualControllerReference(currentCatalogSource, catalogSource)
	specChanged := !equality.Semantic.DeepEqual(catalogSource.Spec, currentCatalogSource.Spec)
	if specChanged || !ownedByAddon {
		// TODO: remove this condition once resourceAdoptionStrategy is discontinued
		if strategy != addonsv1alpha1.ResourceAdoptionAdoptAll && !ownedByAddon {
			return nil, controllers.ErrNotOwnedByUs
		}
		// copy new spec into existing object and update in the k8s api
		currentCatalogSource.Spec = catalogSource.Spec
		currentCatalogSource.OwnerReferences = catalogSource.OwnerReferences
		return currentCatalogSource, c.Update(ctx, currentCatalogSource)
	}

	return currentCatalogSource, nil
}
