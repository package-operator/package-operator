package objectsets

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// objectSliceLoadReconciler loads ObjectSlices to inline all objects into the ObjectSet again.
type objectSliceLoadReconciler struct {
	scheme         *runtime.Scheme
	reader         client.Reader
	newObjectSlice genericObjectSliceFactory
}

func newObjectSliceLoadReconciler(
	scheme *runtime.Scheme,
	reader client.Reader,
	newObjectSlice genericObjectSliceFactory,
) *objectSliceLoadReconciler {
	return &objectSliceLoadReconciler{
		scheme:         scheme,
		reader:         reader,
		newObjectSlice: newObjectSlice,
	}
}

func (r *objectSliceLoadReconciler) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (res ctrl.Result, err error) {
	phases := objectSet.GetPhases()
	for i := range phases {
		phase := &phases[i]
		for _, slice := range phase.Slices {
			objSlice := r.newObjectSlice(r.scheme)
			if err := r.reader.Get(ctx, client.ObjectKey{
				Name:      slice,
				Namespace: objectSet.ClientObject().GetNamespace(),
			}, objSlice.ClientObject()); err != nil {
				return res, fmt.Errorf("getting ObjectSlice: %w", err)
			}

			phase.Objects = append(phase.Objects, objSlice.GetObjects()...)
		}
	}
	objectSet.SetPhases(phases)
	return
}
