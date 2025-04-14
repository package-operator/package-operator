package objectsets

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/adapters"
)

const revisionReconcilerRequeueDelay = 10 * time.Second

// revisionReconciler determines the .status.revision number by checking previous revisions.
type revisionReconciler struct {
	scheme       *runtime.Scheme
	newObjectSet adapters.ObjectSetAccessorFactory
	client       client.Client
}

func (r *revisionReconciler) Reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (res ctrl.Result, err error) {
	if objectSet.GetRevision() != 0 {
		// .status.revision is already set.
		return
	}

	if len(objectSet.GetPrevious()) == 0 {
		// no previous revision(s) specified, default to revision 1
		objectSet.SetRevision(1)
		return
	}

	// Determine new revision number by inspecting previous revisions:
	var latestPreviousRevision int64
	for _, prev := range objectSet.GetPrevious() {
		prevObjectSet := r.newObjectSet(r.scheme)
		key := client.ObjectKey{
			Name:      prev.Name,
			Namespace: objectSet.ClientObject().GetNamespace(),
		}
		if err := r.client.Get(ctx, key, prevObjectSet.ClientObject()); err != nil {
			return res, fmt.Errorf("getting previous revision: %w", err)
		}

		sr := prevObjectSet.GetRevision()
		if sr == 0 {
			logr.FromContextOrDiscard(ctx).
				Info("waiting for previous revision to report revision number", "object", key)
			// retry later
			// this delay is needed, because we are not watching previous revisions from this object
			// which means we are not getting requeued when .status.revision is finally reported.
			res.RequeueAfter = revisionReconcilerRequeueDelay
			return res, nil
		}

		if sr > latestPreviousRevision {
			latestPreviousRevision = sr
		}
	}

	objectSet.SetRevision(latestPreviousRevision + 1)
	if err := r.client.Status().Update(ctx, objectSet.ClientObject()); err != nil {
		return res, fmt.Errorf("update revision in status: %w", err)
	}

	return
}
