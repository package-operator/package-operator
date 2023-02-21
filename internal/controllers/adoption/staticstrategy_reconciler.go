package adoption

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "package-operator.run/apis/coordination/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
)

type StaticAdoptionReconciler struct {
	client       client.Writer
	dynamicCache client.Reader
}

func newStaticAdoptionReconciler(
	client client.Writer,
	dynamicCache client.Reader,
) *StaticAdoptionReconciler {
	return &StaticAdoptionReconciler{
		client:       client,
		dynamicCache: dynamicCache,
	}
}

func (r *StaticAdoptionReconciler) Reconcile(
	ctx context.Context, adoption genericAdoption,
) (ctrl.Result, error) {
	if adoption.GetStrategyType() != coordinationv1alpha1.AdoptionStrategyStatic {
		// noop, a different strategy will match.
		return ctrl.Result{}, nil
	}

	specLabels := adoption.GetStaticStrategy().Labels
	selector, err := negativeLabelKeySelectorFromLabels(specLabels)
	if err != nil {
		return ctrl.Result{}, err
	}

	gvk, _, objListType := controllers.UnstructuredFromTargetAPI(adoption.GetTargetAPI())

	// List all the things not yet labeled.
	if err := r.dynamicCache.List(
		ctx, objListType,
		client.InNamespace(adoption.ClientObject().GetNamespace()), // can also set this for ClusterAdoption without issue.
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

	meta.SetStatusCondition(adoption.GetConditions(), metav1.Condition{
		Type:   coordinationv1alpha1.AdoptionActive,
		Status: metav1.ConditionTrue,
		Reason: "StaticStrategyApplied",
	})

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
