package boxcutterutil

import (
	"context"

	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/managedcache"
	"pkg.package-operator.run/boxcutter/validation"
)

type RevisionEngine interface {
	Reconcile(
		ctx context.Context, rev types.Revision,
		opts ...types.RevisionReconcileOption,
	) (machinery.RevisionResult, error)
	Teardown(
		ctx context.Context, rev types.Revision,
		opts ...types.RevisionTeardownOption,
	) (machinery.RevisionTeardownResult, error)
}

type RevisionEngineFactory interface {
	New(managedcache.Accessor) (RevisionEngine, error)
}

type revisionEngineFactory struct {
	phaseEngineFactory PhaseEngineFactory
	revisionValidator  *validation.RevisionValidator
}

func NewRevisionEngineFactory(
	phaseEngineFactory PhaseEngineFactory,
	revisionValidator *validation.RevisionValidator,
) RevisionEngineFactory {
	return revisionEngineFactory{
		phaseEngineFactory: phaseEngineFactory,
		revisionValidator:  revisionValidator,
	}
}

func (f revisionEngineFactory) New(accessor managedcache.Accessor) (RevisionEngine, error) {
	pe, err := f.phaseEngineFactory.New(accessor)
	if err != nil {
		return nil, err
	}
	return machinery.NewRevisionEngine(pe, f.revisionValidator, accessor), nil
}
