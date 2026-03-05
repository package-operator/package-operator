package controllers

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type PreviousRevisionLookup struct {
	scheme               *runtime.Scheme
	newPreviousObjectSet PreviousObjectSetFactory
	client               client.Reader
}

func NewPreviousRevisionLookup(
	scheme *runtime.Scheme,
	newPreviousObjectSet PreviousObjectSetFactory,
	client client.Reader,
) *PreviousRevisionLookup {
	return &PreviousRevisionLookup{
		scheme:               scheme,
		newPreviousObjectSet: newPreviousObjectSet,
		client:               client,
	}
}

type PreviousOwner interface {
	ClientObject() client.Object
	GetSpecPrevious() []corev1alpha1.PreviousRevisionReference
}

type PreviousObjectSet interface {
	ClientObject() client.Object
	GetStatusRemotePhases() []corev1alpha1.RemotePhaseReference
}

type PreviousObjectSetFactory func(*runtime.Scheme) PreviousObjectSet

func (l *PreviousRevisionLookup) Lookup(
	ctx context.Context, owner PreviousOwner,
) ([]PreviousObjectSet, error) {
	previous := owner.GetSpecPrevious()
	previousSets := make([]PreviousObjectSet, len(previous))
	for i, prev := range previous {
		set := l.newPreviousObjectSet(l.scheme)
		err := l.client.Get(
			ctx, client.ObjectKey{
				Name:      prev.Name,
				Namespace: owner.ClientObject().GetNamespace(),
			}, set.ClientObject())
		// Previous revisions may be garbage collected so ingore not found errors
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		previousSets[i] = set
	}
	return previousSets, nil
}

func (l *PreviousRevisionLookup) LookupPreviousRemotePhases(
	ctx context.Context, owner PreviousOwner,
) ([]client.Object, error) {
	remotePhases := make([]client.Object, 0)
	previousObjSets, err := l.Lookup(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("looking up previous objectsetphases: %w", err)
	}

	for _, prev := range previousObjSets {
		remotePhaseReferences := prev.GetStatusRemotePhases()
		if len(remotePhaseReferences) == 0 {
			continue
		}

		prevGVK, err := apiutil.GVKForObject(prev.ClientObject(), l.scheme)
		if err != nil {
			panic(err)
		}

		phases := l.lookupRemotePhases(remotePhaseReferences, prevGVK, prev.ClientObject().GetNamespace())
		remotePhases = append(remotePhases, phases...)
	}

	return remotePhases, nil
}

func (l *PreviousRevisionLookup) lookupRemotePhases(
	ref []corev1alpha1.RemotePhaseReference, gvk schema.GroupVersionKind, namespace string,
) []client.Object {
	remotePhases := make([]client.Object, 0, len(ref))

	var remoteGVK schema.GroupVersionKind
	if strings.HasPrefix(gvk.Kind, "Cluster") {
		// ClusterObjectSet
		remoteGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectSetPhase")
	} else {
		// ObjectSet
		remoteGVK = corev1alpha1.GroupVersion.WithKind("ObjectSetPhase")
	}
	for _, remote := range ref {
		phase := &unstructured.Unstructured{}
		phase.SetGroupVersionKind(remoteGVK)
		phase.SetName(remote.Name)
		phase.SetUID(remote.UID)
		phase.SetNamespace(namespace)

		remotePhases = append(remotePhases, phase)
	}
	return remotePhases
}
