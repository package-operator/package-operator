package objectdeployments

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/adapters"
)

const defaultRevisionLimit int32 = 10

type archiveReconciler struct {
	client client.Client
}

func (a *archiveReconciler) Reconcile(ctx context.Context,
	currentObjectSet adapters.ObjectSetAccessor,
	prevObjectSets []adapters.ObjectSetAccessor,
	objectDeployment adapters.ObjectDeploymentAccessor,
) (ctrl.Result, error) {
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
	err = a.markObjectSetsForArchival(ctx, prevObjectSets, objsetsEligibleForArchival, objectDeployment)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (a *archiveReconciler) markObjectSetsForArchival(ctx context.Context,
	previousObjectSets []adapters.ObjectSetAccessor,
	objectsToArchive []adapters.ObjectSetAccessor,
	objectDeployment adapters.ObjectDeploymentAccessor,
) error {
	if len(objectsToArchive) == 0 {
		return nil
	}

	// We sort the objectsets to be archived in the increasing order
	// of revision so that the earlier revisions get deleted first.
	sort.Sort(objectSetsByRevisionAscending(objectsToArchive))

	for _, objectSet := range objectsToArchive {
		if !objectSet.IsSpecArchived() && objectSet.IsStatusPaused() {
			objectSet.SetSpecArchived()
			if err := a.client.Update(ctx, objectSet.ClientObject()); err != nil {
				return fmt.Errorf("failed to archive objectset: %w", err)
			}
		}
		// Only garbage collect older revisions if later ones successfully archive
		if err := a.garbageCollectRevisions(ctx, previousObjectSets, objectDeployment); err != nil {
			return fmt.Errorf("error garbage collecting revisions: %w", err)
		}
	}
	return nil
}

func (a *archiveReconciler) objectSetsToBeArchived(
	ctx context.Context,
	allObjectSets []adapters.ObjectSetAccessor,
) ([]adapters.ObjectSetAccessor, error) {
	// Sort all ObjectSets by their ascending revision number.
	sort.Sort(objectSetsByRevisionAscending(allObjectSets))
	objectSetsToArchive := make([]adapters.ObjectSetAccessor, 0)
	for j := len(allObjectSets) - 1; j >= 0; j-- {
		currentLatestRevision := allObjectSets[j]

		// Case 1:
		// currentRevision is "Available",
		// so all previous revisions can be archived.
		if currentLatestRevision.IsSpecAvailable() {
			prevRevisionsToArchive, err := a.archiveAllLaterRevisions(ctx, currentLatestRevision, allObjectSets[:j])
			if err != nil {
				return []adapters.ObjectSetAccessor{}, err
			}
			return append(objectSetsToArchive, prevRevisionsToArchive...), nil
		}

		// If prev revision is present
		if j > 0 {
			previousRevision := allObjectSets[j-1]

			// Already archived dont do anything
			if previousRevision.IsSpecArchived() {
				continue
			}

			if currentLatestRevision.GetSpecRevision() <= previousRevision.GetSpecRevision() {
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
				return []adapters.ObjectSetAccessor{}, err
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
	currentLatest adapters.ObjectSetAccessor,
	laterRevisions []adapters.ObjectSetAccessor,
) ([]adapters.ObjectSetAccessor, error) {
	res := make([]adapters.ObjectSetAccessor, 0)
	for _, currPrev := range laterRevisions {
		// revision already archived, we just skip.
		if currPrev.IsSpecArchived() {
			continue
		}
		// Sanity check
		// We always expect the  currentLatestRevision objectset to have a revision greater
		// than the previous revision
		if currPrev.GetSpecRevision() < currentLatest.GetSpecRevision() {
			IsSpecPaused, err := a.ensurePaused(ctx, currPrev)
			if err != nil {
				return []adapters.ObjectSetAccessor{}, err
			}
			if IsSpecPaused {
				res = append(res, currPrev)
			}
		}
	}
	return res, nil
}

func (a *archiveReconciler) intermediateRevisionCanBeArchived(
	ctx context.Context, previousRevision, currentLatestRevision adapters.ObjectSetAccessor,
) (bool, error) {
	latestRevisionObjects, err := newObjectSetGetter(currentLatestRevision).getObjects()
	if err != nil {
		return false, err
	}
	previousRevisionActivelyReconciledObjects := newObjectSetGetter(previousRevision).getActivelyReconciledObjects()
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
	if len(commonObjects) == 0 && !previousRevision.IsSpecAvailable() {
		// This previousRevision is a candidate for archival but we only
		// proceed to archive it after its paused.
		IsSpecPaused, err := a.ensurePaused(ctx, previousRevision)
		if err != nil {
			return false, err
		}
		return IsSpecPaused, nil
	}
	return false, nil
}

func (a *archiveReconciler) ensurePaused(ctx context.Context, objectset adapters.ObjectSetAccessor) (bool, error) {
	if objectset.IsStatusPaused() {
		return true, nil
	}

	if objectset.IsSpecPaused() {
		// Revision is already marked for pausing but is
		// not yet reflected in its status block
		return false, nil
	}

	// Pause the revision
	objectset.SetSpecPaused()
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

func (a *archiveReconciler) garbageCollectRevisions(
	ctx context.Context,
	previousObjectSets []adapters.ObjectSetAccessor,
	objectDeployment adapters.ObjectDeploymentAccessor,
) error {
	revisionLimit := defaultRevisionLimit
	deploymentRevisionLimit := objectDeployment.GetSpecRevisionHistoryLimit()
	if deploymentRevisionLimit != nil {
		revisionLimit = *deploymentRevisionLimit
	}
	numToDelete := len(previousObjectSets) - int(revisionLimit)
	for _, previousObjectSet := range previousObjectSets {
		if numToDelete <= 0 {
			break
		}

		if err := a.client.Delete(ctx, previousObjectSet.ClientObject()); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete objectset: %w", err)
		}
		numToDelete--
	}

	return nil
}
