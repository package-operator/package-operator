package controllers

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"pkg.package-operator.run/boxcutter/managedcache"
)

type PhaseReconcilerFactory interface {
	New(managedcache.Accessor) PhaseReconciler
}

type phaseReconcilerFactory struct {
	scheme *runtime.Scheme
	// Dangerous: the uncached client here is not scoped by
	// the mapper passed to managedcache.ObjectBoundAccessManager!
	// This warning will be removed when the rest of PKO will be refactored
	// to use boxcutter's {Revision,Phase,Object}Engines.
	uncachedClient   client.Reader
	ownerStrategy    ownerStrategy
	preflightChecker preflightChecker
}

func NewPhaseReconcilerFactory(
	scheme *runtime.Scheme,
	uncachedClient client.Reader,
	ownerStrategy ownerStrategy,
	preflightChecker preflightChecker,
) PhaseReconcilerFactory {
	return phaseReconcilerFactory{
		scheme:           scheme,
		uncachedClient:   uncachedClient,
		ownerStrategy:    ownerStrategy,
		preflightChecker: preflightChecker,
	}
}

func (f phaseReconcilerFactory) New(accessor managedcache.Accessor) PhaseReconciler {
	return &phaseReconciler{
		scheme:   f.scheme,
		accessor: accessor,
		// Dangerous: the uncached client here is not scoped by
		// the mapper passed to managedcache.ObjectBoundAccessManager!
		// This warning will be removed when the rest of PKO will be refactored
		// to use boxcutter's {Revision,Phase,Object}Engines.
		uncachedClient: f.uncachedClient,
		ownerStrategy:  f.ownerStrategy,
		adoptionChecker: &defaultAdoptionChecker{
			ownerStrategy: f.ownerStrategy,
			scheme:        f.scheme,
		},
		patcher:          &defaultPatcher{writer: accessor},
		preflightChecker: f.preflightChecker,
	}
}
