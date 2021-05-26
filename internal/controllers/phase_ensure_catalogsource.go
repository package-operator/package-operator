package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

const catalogSourcePublisher = "OSD Red Hat Addons"

type ensureCatalogSourceResult int

const (
	ensureCatalogSourceResultNil = iota
	ensureCatalogSourceResultStop
	ensureCatalogSourceResultRetry
)

// Ensure existence of the CatalogSource specified in the given Addon resource
// returns an ensureCatalogSourceResult that signals the caller if they have to
// stop or retry reconciliation of the surrounding Addon resource
func (r *AddonReconciler) ensureCatalogSource(
	ctx context.Context, log logr.Logger, addon *addonsv1alpha1.Addon) (ensureCatalogSourceResult, error) {
	var targetNamespace, catalogSourceImage string

	switch addon.Spec.Install.Type {
	case addonsv1alpha1.OwnNamespace:
		if addon.Spec.Install.OwnNamespace == nil ||
			len(addon.Spec.Install.OwnNamespace.Namespace) == 0 {
			// invalid/missing configuration
			// TODO: Move error reporting into webhook and reduce this code to a sanity check.
			return ensureCatalogSourceResultStop, r.reportConfigurationError(ctx, addon, ".spec.install.ownNamespace.namespace is required when .spec.install.type = OwnNamespace")
		}
		targetNamespace = addon.Spec.Install.OwnNamespace.Namespace
		if len(addon.Spec.Install.OwnNamespace.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			// TODO: Move error reporting into webhook and reduce this code to a sanity check.
			return ensureCatalogSourceResultStop, r.reportConfigurationError(ctx, addon, ".spec.install.ownNamespacee.catalogSourceImage is required when .spec.install.type = OwnNamespace")
		}
		catalogSourceImage = addon.Spec.Install.OwnNamespace.CatalogSourceImage

	case addonsv1alpha1.AllNamespaces:
		if addon.Spec.Install.AllNamespaces == nil ||
			len(addon.Spec.Install.AllNamespaces.Namespace) == 0 {
			// invalid/missing configuration
			// TODO: Move error reporting into webhook and reduce this code to a sanity check.
			return ensureCatalogSourceResultStop, r.reportConfigurationError(ctx, addon, ".spec.install.allNamespaces.namespace is required when .spec.install.type = AllNamespaces")
		}
		targetNamespace = addon.Spec.Install.AllNamespaces.Namespace
		if len(addon.Spec.Install.AllNamespaces.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			// TODO: Move error reporting into webhook and reduce this code to a sanity check.
			return ensureCatalogSourceResultStop, r.reportConfigurationError(ctx, addon, ".spec.install.allNamespaces.catalogSourceImage is required when .spec.install.type = AllNamespaces")
		}
		catalogSourceImage = addon.Spec.Install.AllNamespaces.CatalogSourceImage

	default:
		// Unsupported Install Type
		// This should never happen, unless the schema validation is wrong.
		// The .install.type property is set to only allow known enum values.
		log.Error(fmt.Errorf("invalid Addon install type: %q", addon.Spec.Install.Type), "stopping Addon reconcilation")
		return ensureCatalogSourceResultStop, nil
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

	addCommonLabels(catalogSource.Labels, addon)

	if err := controllerutil.SetControllerReference(addon, catalogSource, r.Scheme); err != nil {
		return ensureCatalogSourceResultNil, err
	}

	var observedCatalogSource *operatorsv1alpha1.CatalogSource
	{
		var err error
		observedCatalogSource, err = reconcileCatalogSource(ctx, r.Client, catalogSource)
		if err != nil {
			return ensureCatalogSourceResultNil, err
		}
	}

	if observedCatalogSource.Status.GRPCConnectionState == nil {
		err := r.reportCatalogSourceUnreadinessStatus(ctx, addon, observedCatalogSource, ".Status.GRPCConnectionState is nil")
		if err != nil {
			return ensureCatalogSourceResultNil, err
		}
		return ensureCatalogSourceResultRetry, nil
	}
	if observedCatalogSource.Status.GRPCConnectionState.LastObservedState != "READY" {
		err := r.reportCatalogSourceUnreadinessStatus(
			ctx,
			addon,
			observedCatalogSource,
			fmt.Sprintf(
				".Status.GRPCConnectionState.LastObservedState == %s",
				observedCatalogSource.Status.GRPCConnectionState.LastObservedState,
			),
		)
		if err != nil {
			return ensureCatalogSourceResultNil, err
		}
		return ensureCatalogSourceResultRetry, err
	}

	return ensureCatalogSourceResultNil, nil
}

// Marks Addon as unavailable because the CatalogSource is unready
func (r *AddonReconciler) reportCatalogSourceUnreadinessStatus(
	ctx context.Context,
	addon *addonsv1alpha1.Addon,
	catalogSource *operatorsv1alpha1.CatalogSource,
	message string) error {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:   addonsv1alpha1.Available,
		Status: metav1.ConditionFalse,
		Reason: "UnreadyCatalogSource",
		Message: fmt.Sprintf(
			"CatalogSource connection is not ready: %s",
			message),
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhasePending
	return r.Status().Update(ctx, addon)
}

// reconciles a CatalogSource and returns a new CatalogSource object with observed state.
// Warning: Will adopt existing CatalogSource
func reconcileCatalogSource(ctx context.Context, c client.Client, catalogSource *operatorsv1alpha1.CatalogSource) (
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

	// only update when spec has changed
	if !equality.Semantic.DeepEqual(catalogSource.Spec, currentCatalogSource.Spec) {
		// copy new spec into existing object and update in the k8s api
		currentCatalogSource.Spec = catalogSource.Spec
		return currentCatalogSource, c.Update(ctx, currentCatalogSource)
	}

	return currentCatalogSource, nil
}
