package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	GetPrevious() []corev1alpha1.PreviousRevisionReference
}

type PreviousObjectSet interface {
	ClientObject() client.Object
	GetRemotePhases() []corev1alpha1.RemotePhaseReference
}

type PreviousObjectSetFactory func(*runtime.Scheme) PreviousObjectSet

func (l *PreviousRevisionLookup) Lookup(
	ctx context.Context, owner PreviousOwner,
) ([]PreviousObjectSet, error) {
	previous := owner.GetPrevious()
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
