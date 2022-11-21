package objectdeployments

import (
	"context"
	"fmt"
	"sort"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultRevisionLimit int32 = 10

type archiveReconciler struct {
	client client.Client
}

func (a *archiveReconciler) Reconcile(ctx context.Context,
	currentObjectSet genericObjectSet,
	prevObjectSets []genericObjectSet,
	objectDeployment objectDeploymentAccessor) (ctrl.Result, error) {
	if currentObjectSet == nil {
		return ctrl.Result{}, nil
	}

	objsetsEligibleForArchival, err := a.objectSetsToBeArchived(
		ctx,
		append(prevObjectSets, currentObjectSet),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("errored when trying to compute objects for archival: %w", err)
	}
	return ctrl.Result{}, a.markObjectSetsForArchival(ctx, objsetsEligibleForArchival, objectDeployment)
}

func (a *archiveReconciler) markObjectSetsForArchival(ctx context.Context,
	objectsToArchive []genericObjectSet,
	objectDeployment objectDeploymentAccessor) error {
	if len(objectsToArchive) == 0 {
		return nil
	}

	// We sort the objectsets to be archived in the increasing order
	// of revision so that the earlier revisions get deleted first.
	sort.Sort(objectSetsByRevisionAscending(objectsToArchive))

	revisionLimit := defaultRevisionLimit
	deploymentRevisionLimit := objectDeployment.GetRevisionHistoryLimit()
	if deploymentRevisionLimit != nil {
		revisionLimit = *deploymentRevisionLimit
	}
	numObjectsToDelete := len(objectsToArchive) - int(revisionLimit)
	itemsDeleted := 0
	for _, objectSet := range objectsToArchive {
		currObject := objectSet.ClientObject()
		if numObjectsToDelete > itemsDeleted {
			if err := a.client.Delete(ctx, currObject); err != nil {
				return fmt.Errorf("failed to delete objectset: %w", err)
			}
			itemsDeleted++
			continue
		}

		// Mark everything else as archived
		if !objectSet.IsArchived() && objectSet.IsStatusPaused() {
			objectSet.SetArchived()
			if err := a.client.Update(ctx, objectSet.ClientObject()); err != nil {
				return fmt.Errorf("failed to archive objectset: %w", err)
			}
		}
	}
	return nil
}

func (a *archiveReconciler) objectSetsToBeArchived(
	ctx context.Context,
	allObjectSets []genericObjectSet,
) ([]genericObjectSet, error) {
	// Sort all ObjectSets by their ascending revision number.
	sort.Sort(objectSetsByRevisionAscending(allObjectSets))
	objectSetsToArchive := make([]genericObjectSet, 0)
	for j := len(allObjectSets) - 1; j >= 0; j-- {
		currentLatestRevision := allObjectSets[j]

		// Case 1:
		// currentRevision is "Available",
		// so all previous revisions can be archived.
		if currentLatestRevision.IsAvailable() {
			prevRevisionsToArchive, err := a.archiveAllLaterRevisions(ctx, currentLatestRevision, allObjectSets[:j])
			if err != nil {
				return []genericObjectSet{}, err
			}
			return append(objectSetsToArchive, prevRevisionsToArchive...), nil
		}

		// If prev revision is present
		if j > 0 {
			previousRevision := allObjectSets[j-1]

			// Already archived dont do anything
			if previousRevision.IsArchived() {
				continue
			}

			if currentLatestRevision.GetRevision() <= previousRevision.GetRevision() {
				// Sanity check
				// We always expect the  currentLatestRevision objectset to have a revision greater
				// than the previous revision
				continue
			}

			// Case 2 and 3 handled here:
			shouldArchive, err := a.intermediateRevisionCanBeArchived(
				ctx,
				previousRevision,
				currentLatestRevision,
			)
			if err != nil {
				return []genericObjectSet{}, err
			}
			if shouldArchive {
				objectSetsToArchive = append(objectSetsToArchive, previousRevision)
			}
		}
	}
	return objectSetsToArchive, nil
}

func (a *archiveReconciler) archiveAllLaterRevisions(
	ctx context.Context,
	currentLatest genericObjectSet,
	laterRevisions []genericObjectSet) ([]genericObjectSet, error) {
	res := make([]genericObjectSet, 0)
	for _, currPrev := range laterRevisions {
		// revision already archived, we just skip.
		if currPrev.IsArchived() {
			continue
		}
		// Sanity check
		// We always expect the  currentLatestRevision objectset to have a revision greater
		// than the previous revision
		if currPrev.GetRevision() < currentLatest.GetRevision() {
			isPaused, err := a.ensurePaused(ctx, currPrev)
			if err != nil {
				return []genericObjectSet{}, err
			}
			if isPaused {
				res = append(res, currPrev)
			}
		}
	}
	return res, nil
}

func (a *archiveReconciler) intermediateRevisionCanBeArchived(
	ctx context.Context, previousRevision, currentLatestRevision genericObjectSet) (bool, error) {
	latestRevisionObjects, err := currentLatestRevision.GetObjects()
	if err != nil {
		return false, err
	}
	previousRevisionActivelyReconciledObjects := previousRevision.GetActivelyReconciledObjects()
	// Actively reconciled status is not yet updated
	if previousRevisionActivelyReconciledObjects == nil {
		// Skip for now, this previousRevision's archival candidature will be checked
		// when its controllerOf status block gets updated.
		return false, nil
	}

	// Case 2:
	// If a revision has no actively reconciled objects, it can be marked for archival.
	// Here, if previousRevision has no actively reconciled objects, `commonObjects` will
	// be an empty list and thus it will get marked for archival.

	// Case 3:
	// Latest revision is not containing any object still actively reconciled by an intermediate.
	// Here if the current `latestRevisionObjects`` doesnt contain any objects in the previous
	// revision's actively reconciled object list (previousRevisionActivelyReconciledObjects)
	// then the current previous revision can be marked for archival. (IF the previous revision
	// is not available).
	commonObjects := intersection(latestRevisionObjects, previousRevisionActivelyReconciledObjects)
	if len(commonObjects) == 0 && !previousRevision.IsAvailable() {
		// This previousRevision is a candidate for archival but we only
		// proceed to archive it after its paused.
		isPaused, err := a.ensurePaused(ctx, previousRevision)
		if err != nil {
			return false, err
		}
		return isPaused, nil
	}
	return false, nil
}

func (a *archiveReconciler) ensurePaused(ctx context.Context, objectset genericObjectSet) (bool, error) {
	if objectset.IsStatusPaused() {
		return true, nil
	}

	if objectset.IsSpecPaused() {
		// Revision is already marked for pausing but is
		// not yet reflected in its status block
		return false, nil
	}

	// Pause the revision
	objectset.SetPaused()
	if err := a.client.Update(ctx, objectset.ClientObject()); err != nil {
		return false, fmt.Errorf("failed to pause objectset for archival: %w", err)
	}
	return false, nil
}

func intersection(a, b []objectIdentifier) (res []objectIdentifier) {
	foundItems := make(map[string]struct{})
	for _, item := range a {
		foundItems[item.UniqueIdentifier()] = struct{}{}
	}

	for _, item := range b {
		if _, found := foundItems[item.UniqueIdentifier()]; found {
			res = append(res, item)
		}
	}
	return
}
