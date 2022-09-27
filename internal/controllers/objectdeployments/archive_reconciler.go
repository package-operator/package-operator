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
	objectDeployment genericObjectDeployment) (ctrl.Result, error) {
	if currentObjectSet != nil {
		objsetsEligibleForArchival, err := a.objectSetsToBeArchived(
			ctx,
			append(prevObjectSets, currentObjectSet),
		)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("errored when trying to compute objects for archival: %w", err)
		}
		return ctrl.Result{}, a.markObjectSetsForArchival(ctx, objsetsEligibleForArchival, objectDeployment)
	}
	return ctrl.Result{}, nil
}

func (a *archiveReconciler) markObjectSetsForArchival(ctx context.Context,
	objectsToArchive []genericObjectSet,
	objectDeployment genericObjectDeployment) error {
	if len(objectsToArchive) == 0 {
		return nil
	}

	// We sort the objectsets to be archived in the increasing order
	// of revision so that the earlier revisions get deleted first.
	sort.Sort(objectSetsByRevision(objectsToArchive))

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
		if !objectSet.IsArchived() {
			// Mark everything else as archived
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
	// Sort all the objectsets in the increasing order of revision
	sort.Sort(objectSetsByRevision(allObjectSets))
	objectSetsToArchive := make([]genericObjectSet, 0)
	for j := len(allObjectSets) - 1; j >= 0; j-- {
		currentLatestRevision := allObjectSets[j]

		// Case 1: If currentRevision is available, then all
		// later revisions can be archived.
		if isAvailable(currentLatestRevision) {
			for _, currPrev := range allObjectSets[:j] {
				if currPrev.GetRevision() < currentLatestRevision.GetRevision() {
					// Sanity check
					// We always expect the  currentLatestRevision objectset to have a revision greater
					// than the previous revision
					objectSetsToArchive = append(objectSetsToArchive, currPrev)
				}
			}
			break
		}

		// If prev revision is present
		if j > 0 {
			previousRevision := allObjectSets[j-1]
			if currentLatestRevision.GetRevision() <= previousRevision.GetRevision() {
				// Sanity check
				// We always expect the  currentLatestRevision objectset to have a revision greater
				// than the previous revision
				continue
			}

			latestRevisionObjects, err := currentLatestRevision.GetObjects()

			if err != nil {
				return []genericObjectSet{}, err
			}
			previousRevisionActivelyReconciledObjects := previousRevision.GetActivelyReconciledObjects()
			// Actively reconciled status is not yet updated
			if previousRevisionActivelyReconciledObjects == nil {
				// Skip for now, this previousRevision's archival candidature will be checked
				// when its controllerOf status block gets updated.
				continue
			}

			// Case 2 and 3 handled here:

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
			if len(commonObjects) == 0 && !isAvailable(previousRevision) {
				objectSetsToArchive = append(objectSetsToArchive, previousRevision)
			}
		}
	}
	return objectSetsToArchive, nil
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
