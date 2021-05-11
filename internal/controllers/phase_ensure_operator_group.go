package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Ensures the presense or absense of an OperatorGroup depending on the Addon install type.
func (r *AddonReconciler) ensureOperatorGroup(
	ctx context.Context, log logr.Logger, addon *addonsv1alpha1.Addon) (stop bool, err error) {
	switch addon.Spec.Install.Type {
	case addonsv1alpha1.AllNamespaces:
		// No OperatorGroup needed, Operator is installed for the whole cluster
		// Ensure that we have no left over OperatorGroup.
		return false, r.DeleteAllOf(ctx, &operatorsv1.OperatorGroup{}, client.MatchingLabelsSelector{
			Selector: commonLabelsAsLabelSelector(addon),
		})

	case addonsv1alpha1.OwnNamespaces:
		// continue

	default:
		// Unsupported Install Type
		// This should never happen, unless the schema validation is wrong.
		// The .install.type property is set to only allow known enum values.
		log.Error(fmt.Errorf("invalid Addon install type: %q", addon.Spec.Install.Type), "stopping Addon reconcilation")
		return true, nil
	}

	if addon.Spec.Install.OwnNamespace == nil ||
		len(addon.Spec.Install.OwnNamespace.Namespace) == 0 {
		// invalid/missing configuration
		// TODO: Move error reporting into webhook and reduce this code to a sanity check.
		addon.Status.ObservedGeneration = addon.Generation
		addon.Status.Phase = addonsv1alpha1.PhaseError
		meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
			Type:    addonsv1alpha1.Available,
			Status:  metav1.ConditionFalse,
			Reason:  "ConfigurationError",
			Message: ".spec.install.ownNamespace.namespace is required when .spec.install.type = OwnNamespace",
		})
		return true, r.Status().Update(ctx, addon)
	}

	desiredOperatorGroup := &operatorsv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addon.Name,
			Namespace: addon.Spec.Install.OwnNamespace.Namespace,
			Labels:    map[string]string{},
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{addon.Spec.Install.OwnNamespace.Namespace},
		},
	}
	addCommonLabels(desiredOperatorGroup.Labels, addon)
	if err := controllerutil.SetControllerReference(addon, desiredOperatorGroup, r.Scheme); err != nil {
		return false, fmt.Errorf("setting controller reference: %w", err)
	}

	return false, r.reconcileOperatorGroup(ctx, desiredOperatorGroup)
}

// Reconciles the Spec of the given OperatorGroup if needed by updating or creating the OperatorGroup.
// The given OperatorGroup is updated to reflect the latest state from the kube-apiserver.
func (r *AddonReconciler) reconcileOperatorGroup(
	ctx context.Context, operatorGroup *operatorsv1.OperatorGroup) error {
	currentOperatorGroup := &operatorsv1.OperatorGroup{}

	err := r.Get(ctx, client.ObjectKeyFromObject(operatorGroup), currentOperatorGroup)
	if errors.IsNotFound(err) {
		return r.Create(ctx, operatorGroup)
	}
	if err != nil {
		return fmt.Errorf("getting OperatorGroup: %w", err)
	}

	if !equality.Semantic.DeepEqual(currentOperatorGroup.Spec, operatorGroup.Spec) {
		return r.Update(ctx, operatorGroup)
	}
	return nil
}
