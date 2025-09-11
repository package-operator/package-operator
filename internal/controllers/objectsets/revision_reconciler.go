package objectsets

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

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
	if objectSet.GetSpecRevision() != 0 {
		// Prioritize .spec.revision set by ObjectDeploymentController's newRevisionReconciler
		objectSet.SetStatusRevision(objectSet.GetSpecRevision())
		return
	}

	if objectSet.GetStatusRevision() != 0 {
		// Update existing ObjectSets to include .spec.revision
		// to phase in new revision numbering approach.
		objectSet.SetSpecRevision(objectSet.GetStatusRevision())
		if err = r.client.Update(ctx, objectSet.ClientObject()); err != nil {
			return res, fmt.Errorf("update revision in spec: %w", err)
		}

		return
	}

	if len(objectSet.GetSpecPrevious()) == 0 {
		// no previous revision(s) specified, default to revision 1
		objectSet.SetStatusRevision(1)
		return
	}

	// Determine new revision number by inspecting previous revisions:
	var latestPreviousRevision int64
	for _, prev := range objectSet.GetSpecPrevious() {
		prevObjectSet := r.newObjectSet(r.scheme)
		key := client.ObjectKey{
			Name:      prev.Name,
			Namespace: objectSet.ClientObject().GetNamespace(),
		}
		err := r.client.Get(ctx, key, prevObjectSet.ClientObject())
		if errors.IsNotFound(err) {
			// Skip deleted revisions in the revision counter
			continue
		}
		if err != nil {
			return res, fmt.Errorf("getting previous revision: %w", err)
		}

		sr := prevObjectSet.GetStatusRevision()
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

	objectSet.SetStatusRevision(latestPreviousRevision + 1)
	if err := r.client.Status().Update(ctx, objectSet.ClientObject()); err != nil {
		return res, fmt.Errorf("update revision in status: %w", err)
	}

	return
}
