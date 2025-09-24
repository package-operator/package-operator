package objectsets

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/adapters"
)

// revisionReconciler determines the .status.revision number by checking previous revisions.
type revisionReconciler struct {
	scheme       *runtime.Scheme
	newObjectSet adapters.ObjectSetAccessorFactory
	client       client.Client
}

func (r *revisionReconciler) Reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (res ctrl.Result, err error) {
	if objectSet.GetStatusRevision() == objectSet.GetSpecRevision() { //nolint:staticcheck
		// .status.revision is already set.
		return
	}

	objectSet.SetStatusRevision(objectSet.GetSpecRevision()) //nolint:staticcheck
	if err := r.client.Status().Update(ctx, objectSet.ClientObject()); err != nil {
		return res, fmt.Errorf("update revision in status: %w", err)
	}
	return
}
