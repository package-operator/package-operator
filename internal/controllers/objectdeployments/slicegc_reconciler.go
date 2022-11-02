package objectdeployments

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/package-operator/internal/controllers"
)

// garbage collects ObjectSlices that are no longer in use.
type sliceGCReconciler struct {
	client             client.Client
	scheme             *runtime.Scheme
	newObjectSliceList genericObjectSliceListFactory
}

func (r *sliceGCReconciler) Reconcile(ctx context.Context,
	currentObjectSet genericObjectSet,
	prevObjectSets []genericObjectSet,
	objectDeployment genericObjectDeployment) (ctrl.Result, error) {
	controllerFieldValue, err := controllers.
		ControllerFieldIndexValue(
			objectDeployment.ClientObject(),
			r.scheme,
		)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("controller index field value: %w", err)
	}

	// List of ObjectSlice names that are currently in-use.
	referencedSlices := map[string]struct{}{}
	for _, phase := range objectDeployment.GetObjectSetTemplate().Spec.Phases {
		for _, slice := range phase.Slices {
			referencedSlices[slice] = struct{}{}
		}
	}
	for _, objectSet := range append(
		[]genericObjectSet{currentObjectSet}, prevObjectSets...) {
		for _, phase := range objectSet.GetPhases() {
			for _, slice := range phase.Slices {
				referencedSlices[slice] = struct{}{}
			}
		}
	}

	// List all Slices controlled by this Deployment.
	controlledSlicesList := r.newObjectSliceList(r.scheme)
	if err := r.client.List(
		ctx, controlledSlicesList.ClientObjectList(),
		client.MatchingFields{
			controllers.ControllerIndexFieldKey: controllerFieldValue,
		},
		client.InNamespace(
			objectDeployment.ClientObject().GetNamespace()),
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing all controlled slices: %w", err)
	}

	// Delete Slices not referenced by any ObjectSet.
	for _, slice := range controlledSlicesList.GetItems() {
		if _, referenced := referencedSlices[slice.ClientObject().GetName()]; referenced {
			continue
		}

		// Slice is not referenced anymore.
		if err := r.client.Delete(ctx, slice.ClientObject()); err != nil {
			return ctrl.Result{}, fmt.Errorf("garbage collect ObjectSlice: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
