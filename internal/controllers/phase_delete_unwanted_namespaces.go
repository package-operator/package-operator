package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Ensure cleanup of namespaces that are not needed anymore for the given addon resource
func (r *AddonReconciler) ensureDeletionOfUnwantedNamespaces(
	ctx context.Context, addon *addonsv1alpha1.Addon) error {
	currentNamespaces, err := getOwnedNamespacesViaCommonLabels(ctx, r.Client, addon)
	if err != nil {
		return err
	}

	wantedNamespaceNames := make(map[string]struct{})
	for _, namespace := range addon.Spec.Namespaces {
		wantedNamespaceNames[namespace.Name] = struct{}{}
	}

	for _, namespace := range currentNamespaces {
		_, isWanted := wantedNamespaceNames[namespace.Name]
		if isWanted {
			// don't delete
			continue
		}

		err := ensureNamespaceDeletion(ctx, r.Client, namespace.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

// Ensure that the given namespace is deleted
func ensureNamespaceDeletion(ctx context.Context, c client.Client, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := c.Delete(ctx, namespace)
	// don't propagate error if the namespace is already gone
	if !k8sApiErrors.IsNotFound(err) {
		return err
	}
	return nil
}

// Get all namespaces that have common labels matching the given addon resource
func getOwnedNamespacesViaCommonLabels(
	ctx context.Context, c client.Client, addon *addonsv1alpha1.Addon) ([]corev1.Namespace, error) {
	selector := commonLabelsAsLabelSelector(addon)

	list := &corev1.NamespaceList{}
	{
		err := c.List(ctx, list, &client.ListOptions{
			LabelSelector: client.MatchingLabelsSelector{
				Selector: selector,
			}})
		if err != nil {
			return nil, fmt.Errorf("could not list owned namespaces: %w", err)
		}
	}

	return list.Items, nil
}
