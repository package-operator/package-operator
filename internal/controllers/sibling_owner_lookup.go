package controllers

import (
	"context"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/adapters"
)

// Used to filter ObjectSets by their owning ObjectDeployment.
const DeploymentLabel = "package-operator.run/object-deployment"

// SiblingOwnerClassifier determines whether an owner reference
// belongs to a sibling of the same deployment.
type SiblingOwnerClassifier func(ownerRef metav1.OwnerReference) bool

// SiblingOwnerLookup discovers sibling ObjectSets of the same deployment
// and builds classifiers for owner reference recognition.
type SiblingOwnerLookup struct {
	scheme           *runtime.Scheme
	client           client.Reader
	newObjectSet     adapters.ObjectSetAccessorFactory
	newObjectSetList adapters.ObjectSetListAccessorFactory
}

func NewSiblingOwnerLookup(
	scheme *runtime.Scheme,
	client client.Reader,
	newObjectSet adapters.ObjectSetAccessorFactory,
	newObjectSetList adapters.ObjectSetListAccessorFactory,
) *SiblingOwnerLookup {
	return &SiblingOwnerLookup{
		scheme:           scheme,
		client:           client,
		newObjectSet:     newObjectSet,
		newObjectSetList: newObjectSetList,
	}
}

// ClassifierForObjectSet returns a classifier that identifies owner references
// belonging to sibling ObjectSets of the same deployment.
// The ObjectSet's own UID and its remote phase UIDs are excluded from the
// classifier — they belong to the current revision, not to siblings.
// Sibling discovery requires the ObjectSet to carry the deployment label
// (package-operator.run/object-deployment). Without it, an empty classifier is
// returned — standalone ObjectSet users must set this label for cross-revision
// handover to work.
func (l *SiblingOwnerLookup) ClassifierForObjectSet(
	ctx context.Context, objectSet adapters.ObjectSetAccessor,
) (SiblingOwnerClassifier, error) {
	deploymentName, hasLabel := objectSet.ClientObject().GetLabels()[DeploymentLabel]
	if !hasLabel {
		return func(metav1.OwnerReference) bool { return false }, nil
	}

	objectSetList := l.newObjectSetList(l.scheme)
	if err := l.client.List(ctx, objectSetList.ClientObjectList(),
		client.MatchingLabels{DeploymentLabel: deploymentName},
		client.InNamespace(objectSet.ClientObject().GetNamespace()),
	); err != nil {
		return nil, fmt.Errorf("listing sibling ObjectSets: %w", err)
	}

	return classifierFromObjectSets(
		objectSetList.GetItems(), objectSet.ClientObject().GetUID(),
	), nil
}

// ClassifierForObjectSetPhase returns a classifier for the parent ObjectSet's
// siblings. Resolves the parent ObjectSet via the controller owner reference
// on the ObjectSetPhase, then delegates to ClassifierForObjectSet.
func (l *SiblingOwnerLookup) ClassifierForObjectSetPhase(
	ctx context.Context, objectSetPhase adapters.ObjectSetPhaseAccessor,
) (SiblingOwnerClassifier, error) {
	ospObj := objectSetPhase.ClientObject()

	controllerRef := metav1.GetControllerOf(ospObj)
	// Short circuit if there is no owning ObjectSet
	//  assume there cannot be any siblings.
	if controllerRef == nil {
		return func(metav1.OwnerReference) bool { return false }, nil
	}

	parentOS := l.newObjectSet(l.scheme)
	if err := l.client.Get(ctx, client.ObjectKey{
		Name:      controllerRef.Name,
		Namespace: ospObj.GetNamespace(),
	}, parentOS.ClientObject()); err != nil {
		return nil, fmt.Errorf("getting parent ObjectSet: %w", err)
	}

	return l.ClassifierForObjectSet(ctx, parentOS)
}

func classifierFromObjectSets(
	objectSets []adapters.ObjectSetAccessor, selfUID types.UID,
) SiblingOwnerClassifier {
	uids := make([]types.UID, 0, len(objectSets))
	for _, objSet := range objectSets {
		uid := objSet.ClientObject().GetUID()
		// Skip current ObjectSet and its ObjectSetPhases.
		if selfUID == uid {
			continue
		}

		uids = append(uids, uid)

		for _, remote := range objSet.GetStatusRemotePhases() {
			uids = append(uids, remote.UID)
		}
	}

	return func(ownerRef metav1.OwnerReference) bool {
		return slices.Contains(uids, ownerRef.UID)
	}
}
