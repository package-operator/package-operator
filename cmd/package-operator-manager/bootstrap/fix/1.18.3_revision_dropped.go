package fix

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
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
		return false, fmt.Errorf("failed to list ObjectSets: %w", err)
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
	if err := f.run(ctx, fc, adapters.NewObjectSetList); err != nil {
		return fmt.Errorf("recovering ObjectSet .spec.revision: %w", err)
	}
	if err := f.run(ctx, fc, adapters.NewClusterObjectSetList); err != nil {
		return fmt.Errorf("recovering ClusterObjectSet .spec.revision: %w", err)
	}
	return nil
}

func (f RevisionDroppedFix) run(
	ctx context.Context, fc Context,
	newObjectSetList adapters.ObjectSetListAccessorFactory,
) (err error) {
	finished := true

	for {
		objectSetList := newObjectSetList(fc.Client.Scheme())
		if err := fc.Client.List(ctx, objectSetList.ClientObjectList()); err != nil {
			return fmt.Errorf("failed to list ObjectSets: %w", err)
		}
		for _, objectSet := range objectSetList.GetItems() {
			if objectSet.GetSpecRevision() != 0 {
				continue
			}
			s, err := f.reconcile(ctx, objectSet, fc, adapters.NewObjectSet)
			if err != nil {
				return err
			}
			if !s {
				finished = false
			}
		}

		if finished {
			break
		}
	}

	return nil
}

func (f *RevisionDroppedFix) reconcile(
	ctx context.Context, objectSet adapters.ObjectSetAccessor, fc Context,
	newObjectSet adapters.ObjectSetAccessorFactory,
) (success bool, err error) {
	if objectSet.GetSpecRevision() != 0 {
		// .status.revision is already set.
		return
	}

	if len(objectSet.GetSpecPrevious()) == 0 {
		// no previous revision(s) specified, default to revision 1
		objectSet.SetSpecRevision(1)
		return
	}

	// Determine new revision number by inspecting previous revisions:
	var latestPreviousRevision int64
	for _, prev := range objectSet.GetSpecPrevious() {
		prevObjectSet := newObjectSet(fc.Client.Scheme())
		key := client.ObjectKey{
			Name:      prev.Name,
			Namespace: objectSet.ClientObject().GetNamespace(),
		}
		if err := fc.Client.Get(ctx, key, prevObjectSet.ClientObject()); err != nil {
			return false, fmt.Errorf("getting previous revision: %w", err)
		}

		sr := prevObjectSet.GetSpecRevision()
		if sr == 0 {
			logr.FromContextOrDiscard(ctx).
				Info("waiting for previous revision to report revision number", "object", key)
			// retry later
			return false, nil
		}

		if sr > latestPreviousRevision {
			latestPreviousRevision = sr
		}
	}

	objectSet.SetSpecRevision(latestPreviousRevision + 1)
	if err := fc.Client.Update(ctx, objectSet.ClientObject(), client.FieldOwner(constants.FieldOwner)); err != nil {
		return false, fmt.Errorf("update revision in spec: %w", err)
	}

	return true, nil
}
