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

type clientWriters interface {
	client.Writer
	client.StatusClient
}

type RoundRobinAdoptionReconciler struct {
	client       clientWriters
	dynamicCache client.Reader
}

func newRoundRobinAdoptionReconciler(
	client clientWriters,
	dynamicCache client.Reader,
) *RoundRobinAdoptionReconciler {
	return &RoundRobinAdoptionReconciler{
		client:       client,
		dynamicCache: dynamicCache,
	}
}

func (r *RoundRobinAdoptionReconciler) Reconcile(
	ctx context.Context, adoption genericAdoption,
) (ctrl.Result, error) {
	if adoption.GetStrategyType() != coordinationv1alpha1.AdoptionStrategyRoundRobin {
		adoption.SetRoundRobinStatus(nil)
		// noop, a different strategy will match.
		return ctrl.Result{}, nil
	}

	roundRobinSpec := adoption.GetRoundRobinSpec()
	selector, err := roundRobinNegativeLabelSelector(roundRobinSpec)
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

	var rrIndex int
	for i := range objListType.Items {
		obj := objListType.Items[i]
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for k, v := range roundRobinSpec.Always {
			labels[k] = v
		}

		// choose an option.
		lastIndex := getLastRoundRobinIndex(adoption)
		rrIndex = roundRobinIndex(lastIndex, len(roundRobinSpec.Options)-1)
		for k, v := range roundRobinSpec.Options[rrIndex] {
			labels[k] = v
		}

		obj.SetLabels(labels)
		if err := r.updateObject(ctx, &obj, adoption, lastIndex); err != nil {
			return ctrl.Result{}, err
		}

		// track last committed index.
		adoption.SetRoundRobinStatus(&coordinationv1alpha1.AdoptionRoundRobinStatus{
			LastIndex: rrIndex,
		})
	}

	meta.SetStatusCondition(adoption.GetConditions(), metav1.Condition{
		Type:   coordinationv1alpha1.AdoptionActive,
		Status: metav1.ConditionTrue,
		Reason: "RoundRobinStrategyApplied",
	})

	return ctrl.Result{}, nil
}

func (r *RoundRobinAdoptionReconciler) updateObject(
	ctx context.Context, obj client.Object, adoption genericAdoption, lastIndex int,
) error {
	if err := r.client.Update(ctx, obj); err != nil {
		// try to save the last committed index.
		adoption.SetRoundRobinStatus(&coordinationv1alpha1.AdoptionRoundRobinStatus{
			LastIndex: lastIndex,
		})
		_ = r.client.Status().Update(ctx, adoption.ClientObject())
		return fmt.Errorf("setting labels: %w", err)
	}
	return nil
}

func roundRobinIndex(lastIndex int, max int) int {
	index := lastIndex + 1
	if index > max {
		return 0
	}
	return index
}

func getLastRoundRobinIndex(adoption genericAdoption) int {
	rr := adoption.GetRoundRobinStatus()
	if rr != nil {
		return rr.LastIndex
	}
	return -1 // no last index.
}

// Builds a labelSelector EXCLUDING all objects that could be targeted.
func roundRobinNegativeLabelSelector(
	roundRobin coordinationv1alpha1.AdoptionStrategyRoundRobinSpec,
) (labels.Selector, error) {
	var requirements []labels.Requirement

	// Commented out:
	// If any of the "always"-labels already exists,
	// it has no effect on the round robin distribution.
	//
	// for k := range roundRobin.Always {
	// 	requirement, err := labels.NewRequirement(
	// 		k, selection.DoesNotExist, nil)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("building requirement: %w", err)
	// 	}
	// 	requirements = append(requirements, *requirement)
	// }

	for i := range roundRobin.Options {
		for k := range roundRobin.Options[i] {
			requirement, err := labels.NewRequirement(
				k, selection.DoesNotExist, nil)
			if err != nil {
				return nil, fmt.Errorf("building requirement: %w", err)
			}
			requirements = append(requirements, *requirement)
		}
	}

	selector := labels.NewSelector().Add(requirements...)
	return selector, nil
}
