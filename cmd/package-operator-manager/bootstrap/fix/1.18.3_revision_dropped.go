package fix

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
)

// RevisionDroppedFix sets .spec.revision if it is 0.
// .spec.revision has been introduced in PKO 1.18.3 and replaces .status.revision.
// Previously existing ObjectSets don't have their revision field populated.
// The ObjectDeployment reconciler waits until all ObjectSets reported a revision
// before creating new ones leading to a deadlock.
type RevisionDroppedFix struct{}

func (f RevisionDroppedFix) Check(ctx context.Context, fc Context) (bool, error) {
	objectSetList := &corev1alpha1.ObjectSetList{}
	if err := fc.Client.List(ctx, objectSetList); err != nil {
		return false, fmt.Errorf("failed to list ObjectSets: %w", err)
	}
	for _, objectSet := range objectSetList.Items {
		if objectSet.Spec.Revision == 0 {
			// Missing Revision
			return true, nil
		}
	}

	clusterObjectSetList := &corev1alpha1.ClusterObjectSetList{}
	if err := fc.Client.List(ctx, clusterObjectSetList); err != nil {
		return false, fmt.Errorf("failed to list ClusterObjectSets: %w", err)
	}
	for _, clusterObjectSet := range clusterObjectSetList.Items {
		if clusterObjectSet.Spec.Revision == 0 {
			// Missing Revision
			return true, nil
		}
	}

	return false, nil
}

func (f RevisionDroppedFix) Run(ctx context.Context, fc Context) error {
retry:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			successObjectSet, err := f.run(ctx, fc, adapters.NewObjectSetList, adapters.NewObjectSet)
			if err != nil {
				return fmt.Errorf("recovering ObjectSet .spec.revision: %w", err)
			}
			successClusterObjectSet, err := f.run(ctx, fc, adapters.NewClusterObjectSetList, adapters.NewClusterObjectSet)
			if err != nil {
				return fmt.Errorf("recovering ClusterObjectSet .spec.revision: %w", err)
			}
			if successObjectSet && successClusterObjectSet {
				break retry
			}
		}
	}
	return nil
}

func (f RevisionDroppedFix) run(
	ctx context.Context, fc Context,
	newObjectSetList adapters.ObjectSetListAccessorFactory,
	newObjectSet adapters.ObjectSetAccessorFactory,
) (finished bool, err error) {
	finished = true

	objectSetList := newObjectSetList(fc.Client.Scheme())
	if err := fc.Client.List(ctx, objectSetList.ClientObjectList()); err != nil {
		return false, fmt.Errorf("failed to list ObjectSets: %w", err)
	}
	for _, objectSet := range objectSetList.GetItems() {
		if objectSet.GetSpecRevision() != 0 {
			continue
		}
		s, err := f.reconcile(ctx, objectSet, fc, newObjectSet)
		if err != nil {
			return false, err
		}
		if !s {
			finished = false
		}
	}

	return finished, nil
}

func (f *RevisionDroppedFix) reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor, fc Context,
	newObjectSet adapters.ObjectSetAccessorFactory,
) (success bool, err error) {
	if objectSet.GetSpecRevision() != 0 {
		// .spec.revision is already set.
		return true, nil
	}

	latestPreviousRevision, retry, err := f.determineLatestPreviousRevision(ctx, objectSet, fc, newObjectSet)
	if err != nil {
		return false, err
	} else if retry {
		return false, nil
	}

	// Current revision is latest + 1.
	objectSet.SetSpecRevision(latestPreviousRevision + 1)

	// Update revision.
	if err := fc.Client.Update(ctx, objectSet.ClientObject(), client.FieldOwner(constants.FieldOwner)); err != nil {
		return false, fmt.Errorf("update revision in spec: %w", err)
	}

	return true, nil
}

// Determine new revision number by inspecting previous revisions.
// Returns 0 if none found or if no previous revisions are listed.
func (f *RevisionDroppedFix) determineLatestPreviousRevision(
	ctx context.Context, objectSet adapters.ObjectSetAccessor, fc Context,
	newObjectSet adapters.ObjectSetAccessorFactory,
) (revision int64, retry bool, err error) {
	if len(objectSet.GetSpecPrevious()) == 0 {
		return 0, false, nil
	}

	var latestPreviousRevision int64
	for _, prev := range objectSet.GetSpecPrevious() {
		prevObjectSet := newObjectSet(fc.Client.Scheme())
		key := client.ObjectKey{
			Name:      prev.Name,
			Namespace: objectSet.ClientObject().GetNamespace(),
		}

		err := fc.Client.Get(ctx, key, prevObjectSet.ClientObject())
		if apimachineryerrors.IsNotFound(err) {
			logr.FromContextOrDiscard(ctx).
				Info("previous revision not found, skipping", "object", key)
			continue
		} else if err != nil {
			return 0, false, fmt.Errorf("getting previous revision: %w", err)
		}

		sr := prevObjectSet.GetSpecRevision()
		if sr == 0 {
			logr.FromContextOrDiscard(ctx).
				Info("waiting for previous revision to report revision number", "object", key)
			// retry later
			return 0, true, nil
		}

		if sr > latestPreviousRevision {
			latestPreviousRevision = sr
		}
	}

	return latestPreviousRevision, false, nil
}
