package objectsets

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/ownerhandling"
)

// objectSliceLoadReconciler loads ObjectSlices to inline all objects into the ObjectSet again.
type objectSliceLoadReconciler struct {
	scheme         *runtime.Scheme
	client         client.Client
	newObjectSlice adapters.ObjectSliceFactory
	ownerStrategy  ownerStrategy
}

func newObjectSliceLoadReconciler(
	scheme *runtime.Scheme,
	client client.Client,
	newObjectSlice adapters.ObjectSliceFactory,
) *objectSliceLoadReconciler {
	return &objectSliceLoadReconciler{
		scheme:         scheme,
		client:         client,
		newObjectSlice: newObjectSlice,
		ownerStrategy:  ownerhandling.NewNative(scheme),
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
			if err := r.client.Get(ctx, client.ObjectKey{
				Name:      slice,
				Namespace: objectSet.ClientObject().GetNamespace(),
			}, objSlice.ClientObject()); err != nil {
				return res, fmt.Errorf("getting ObjectSlice: %w", err)
			}

			if !r.ownerStrategy.IsOwner(objectSet.ClientObject(), objSlice.ClientObject()) {
				if err := controllerutil.SetOwnerReference(
					objectSet.ClientObject(), objSlice.ClientObject(), r.scheme); err != nil {
					return res, fmt.Errorf("set ObjectSlice OwnerReference: %w", err)
				}

				if err := r.client.Update(ctx, objSlice.ClientObject()); err != nil {
					return res, fmt.Errorf("update ObjectSlice OwnerReferences: %w", err)
				}
			}

			phase.Objects = append(phase.Objects, objSlice.GetObjects()...)
		}
	}
	objectSet.SetPhases(phases)
	return
}
