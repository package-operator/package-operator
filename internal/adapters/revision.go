package adapters

import (
	"pkg.package-operator.run/boxcutter/machinery/types"
)

var (
	_ types.Revision        = (*RevisionAdapter)(nil)
	_ types.RevisionBuilder = (*RevisionAdapter)(nil)
)

type RevisionAdapter struct {
	ObjectSet        ObjectSetAccessor
	ReconcileOptions []types.RevisionReconcileOption
	TeardownOptions  []types.RevisionTeardownOption
}

func (r *RevisionAdapter) GetName() string {
	return r.ObjectSet.ClientObject().GetName()
}

func (r *RevisionAdapter) GetRevisionNumber() int64 {
	return r.ObjectSet.GetSpecRevision()
}

func (r *RevisionAdapter) GetPhases() []types.Phase {
	phases := make([]types.Phase, 0, len(r.ObjectSet.GetSpecPhases()))
	for _, p := range r.ObjectSet.GetSpecPhases() {
		phases = append(phases, &PhaseAdapter{
			Phase:     p,
			ObjectSet: r.ObjectSet,
		})
	}
	return phases
}

func (r *RevisionAdapter) GetReconcileOptions() []types.RevisionReconcileOption {
	return r.ReconcileOptions
}

func (r *RevisionAdapter) GetTeardownOptions() []types.RevisionTeardownOption {
	return r.TeardownOptions
}

func (r *RevisionAdapter) WithReconcileOptions(opts ...types.RevisionReconcileOption) types.RevisionBuilder {
	r.ReconcileOptions = append(r.ReconcileOptions, opts...)
	return r
}

func (r *RevisionAdapter) WithTeardownOptions(opts ...types.RevisionTeardownOption) types.RevisionBuilder {
	r.TeardownOptions = append(r.TeardownOptions, opts...)
	return r
}
