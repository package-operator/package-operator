package addon

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
)

// Ensure existence of Namespaces specified in the given Addon resource
// returns a bool that signals the caller to stop reconciliation and retry later
func (r *AddonReconciler) ensureWantedNamespaces(
	ctx context.Context, addon *addonsv1alpha1.Addon) (requeueResult, error) {
	var unreadyNamespaces []string
	var collidedNamespaces []string

	for _, namespace := range addon.Spec.Namespaces {
		ensuredNamespace, err := r.ensureNamespace(ctx, addon, namespace.Name)
		if err != nil {
			if errors.Is(err, controllers.ErrNotOwnedByUs) {
				collidedNamespaces = append(collidedNamespaces, namespace.Name)
				continue
			}
			return resultNil, err
		}

		if ensuredNamespace.Status.Phase != corev1.NamespaceActive {
			unreadyNamespaces = append(unreadyNamespaces, ensuredNamespace.Name)
		}
	}

	if len(collidedNamespaces) > 0 {
		reportCollidedNamespaces(addon, unreadyNamespaces)
		// collisions occured: signal caller to stop and retry
		return resultRetry, nil
	}

	if len(unreadyNamespaces) > 0 {
		reportUnreadyNamespaces(addon, unreadyNamespaces)
		return resultNil, nil
	}

	return resultNil, nil
}

// Ensure a single Namespace for the given Addon resource
func (r *AddonReconciler) ensureNamespace(ctx context.Context, addon *addonsv1alpha1.Addon, name string) (*corev1.Namespace, error) {
	return r.ensureNamespaceWithLabels(ctx, addon, name, map[string]string{})
}

// Ensure a single Namespace with a set of labels for the given Addon resource
func (r *AddonReconciler) ensureNamespaceWithLabels(ctx context.Context, addon *addonsv1alpha1.Addon, name string, labels map[string]string) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
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

	if err := c.Get(ctx, client.ObjectKey{Name: namespace.Name}, currentNamespace); k8sApiErrors.IsNotFound(err) {
		return namespace, c.Create(ctx, namespace)
	} else if err != nil {
		return nil, err
	}

	mustAdopt := len(currentNamespace.OwnerReferences) == 0 ||
		!controllers.HasEqualControllerReference(currentNamespace, namespace)

	// TODO: remove this condition once resourceAdoptionStrategy is discontinued
	if mustAdopt && strategy != addonsv1alpha1.ResourceAdoptionAdoptAll {
		return nil, controllers.ErrNotOwnedByUs
	}

	for k, v := range namespace.Labels {
		currentNamespace.Labels[k] = v
	}
	currentNamespace.OwnerReferences = namespace.OwnerReferences

	return currentNamespace, c.Update(ctx, currentNamespace)
}
