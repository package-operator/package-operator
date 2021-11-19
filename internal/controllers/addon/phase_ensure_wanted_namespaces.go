package addon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
)

// Ensure existence of Namespaces specified in the given Addon resource
// returns a bool that signals the caller to stop reconciliation and retry later
func (r *AddonReconciler) ensureWantedNamespaces(
	ctx context.Context, addon *addonsv1alpha1.Addon) (stopAndRetry bool, err error) {
	var unreadyNamespaces []string
	var collidedNamespaces []string

	for _, namespace := range addon.Spec.Namespaces {
		ensuredNamespace, err := r.ensureNamespace(ctx, addon, namespace.Name)
		if err != nil {
			if errors.Is(err, controllers.ErrNotOwnedByUs) {
				collidedNamespaces = append(collidedNamespaces, namespace.Name)
				continue
			}

			return false, err
		}

		if ensuredNamespace.Status.Phase != corev1.NamespaceActive {
			unreadyNamespaces = append(unreadyNamespaces, ensuredNamespace.Name)
		}
	}

	if len(collidedNamespaces) > 0 {
		meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
			Type:   addonsv1alpha1.Available,
			Status: metav1.ConditionFalse,
			Reason: addonsv1alpha1.AddonReasonCollidedNamespaces,
			Message: fmt.Sprintf(
				"Namespaces with collisions: %s",
				strings.Join(collidedNamespaces, ", ")),
			ObservedGeneration: addon.Generation,
		})
		addon.Status.ObservedGeneration = addon.Generation
		addon.Status.Phase = addonsv1alpha1.PhasePending
		err := r.Status().Update(ctx, addon)
		if err != nil {
			return false, err
		}
		// collisions occured: signal caller to stop and retry
		return true, nil
	}

	if len(unreadyNamespaces) > 0 {
		meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
			Type:   addonsv1alpha1.Available,
			Status: metav1.ConditionFalse,
			Reason: addonsv1alpha1.AddonReasonUnreadyNamespaces,
			Message: fmt.Sprintf(
				"Namespaces not yet in Active phase: %s",
				strings.Join(unreadyNamespaces, ", ")),
			ObservedGeneration: addon.Generation,
		})
		addon.Status.ObservedGeneration = addon.Generation
		addon.Status.Phase = addonsv1alpha1.PhasePending
		return false, r.Status().Update(ctx, addon)
	}

	return false, nil
}

// Ensure a single Namespace for the given Addon resource
func (r *AddonReconciler) ensureNamespace(ctx context.Context, addon *addonsv1alpha1.Addon, name string) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
	}
	controllers.AddCommonLabels(namespace.Labels, addon)
	err := controllerutil.SetControllerReference(addon, namespace, r.Scheme)
	if err != nil {
		return nil, err
	}
	return reconcileNamespace(ctx, r.Client, namespace, addon.Spec.ResourceAdoptionStrategy)
}

// reconciles a Namespace and returns the current object as observed.
// prevents adoption of Namespaces (unowned or owned by something else)
// reconciling a Namespace means: creating it when it is not present
// and erroring if our controller is not the owner of said Namespace
func reconcileNamespace(ctx context.Context, c client.Client,
	namespace *corev1.Namespace, strategy addonsv1alpha1.ResourceAdoptionStrategyType) (*corev1.Namespace, error) {

	currentNamespace := &corev1.Namespace{}

	err := c.Get(ctx, client.ObjectKey{
		Name: namespace.Name,
	}, currentNamespace)

	if k8sApiErrors.IsNotFound(err) {
		return namespace, c.Create(ctx, namespace)
	}
	if err != nil {
		return nil, err
	}

	if len(currentNamespace.OwnerReferences) == 0 ||
		!controllers.HasEqualControllerReference(currentNamespace, namespace) {

		// TODO: remove this condition once resoureceAdoptionStrategy is discontinued
		if strategy == addonsv1alpha1.ResourceAdoptionAdoptAll {
			return namespace, c.Update(ctx, namespace)
		}
		return nil, controllers.ErrNotOwnedByUs
	}
	return currentNamespace, nil
}
