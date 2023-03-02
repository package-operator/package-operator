package handovers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/package-operator/internal/controllers"
)

// Sets the initial label on all unlabeled object instances.
type adoptionReconciler struct {
	client       client.Writer
	dynamicCache client.Reader
}

func newAdoptionReconciler(
	client client.Writer,
	dynamicCache client.Reader,
) *adoptionReconciler {
	return &adoptionReconciler{
		client:       client,
		dynamicCache: dynamicCache,
	}
}

func (r *adoptionReconciler) Reconcile(
	ctx context.Context, handover genericHandover,
) (ctrl.Result, error) {
	specLabels := map[string]string{
		handover.GetRelabelStrategy().LabelKey: handover.GetRelabelStrategy().InitialValue,
	}
	selector, err := negativeLabelKeySelectorFromLabels(specLabels)
	if err != nil {
		return ctrl.Result{}, err
	}

	gvk, _, objListType := controllers.UnstructuredFromTargetAPI(handover.GetTargetAPI())

	// List all the things not yet labeled.
	if err := r.dynamicCache.List(
		ctx, objListType,
		client.InNamespace(handover.ClientObject().GetNamespace()), // can also set this for ClusterHandover without issue.
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing %s: %w", gvk, err)
	}

	for i := range objListType.Items {
		obj := objListType.Items[i]
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for k, v := range specLabels {
			labels[k] = v
		}
		obj.SetLabels(labels)

		if err := r.client.Update(ctx, &obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting labels: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// returns a label selectors that matches all objects without the given label keys set.
func negativeLabelKeySelectorFromLabels(specLabels map[string]string) (labels.Selector, error) {
	// Build selector.
	var requirements []labels.Requirement
	for k := range specLabels {
		requirement, err := labels.NewRequirement(
			k, selection.DoesNotExist, nil)
		if err != nil {
			return nil, fmt.Errorf("building selector: %w", err)
		}
		requirements = append(requirements, *requirement)
	}
	selector := labels.NewSelector().Add(requirements...)
	return selector, nil
}
