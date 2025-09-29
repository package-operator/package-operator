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
	if objectSet.GetStatusRevision() != 0 { //nolint:staticcheck
		// .status.revision is already set.
		return
	}

	if objectSet.GetSpecRevision() != 0 {
		// Prioritize .spec.revision set by ObjectDeploymentController's newRevisionReconciler
		objectSet.SetStatusRevision(objectSet.GetSpecRevision()) //nolint:staticcheck
		return
	}

	// theoretically GetSpecRevision shouldn't return 0 and it should never get here
	log := logr.FromContextOrDiscard(ctx).WithName("revisionReconciler")
	if len(objectSet.GetSpecPrevious()) == 0 {
		// no previous revision(s) specified, default to revision 1
		objectSet.SetStatusRevision(1) //nolint:staticcheck
		log.Error(nil, "no previous revision(s), setting to 1")
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

		sr := prevObjectSet.GetStatusRevision() //nolint:staticcheck
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

	log.Error(nil, ".spec.revision was 0, determined revision from previous revisions",
		"new revision", latestPreviousRevision+1)
	objectSet.SetStatusRevision(latestPreviousRevision + 1) //nolint:staticcheck
	if err := r.client.Status().Update(ctx, objectSet.ClientObject()); err != nil {
		return res, fmt.Errorf("update revision in status: %w", err)
	}

	return
}
