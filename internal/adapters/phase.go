package adapters

import (
	"pkg.package-operator.run/boxcutter/machinery/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var (
	_ types.Phase        = (*PhaseAdapter)(nil)
	_ types.PhaseBuilder = (*PhaseAdapter)(nil)
)

type PhaseAdapter struct {
	Phase            corev1alpha1.ObjectSetTemplatePhase
	ReconcileOptions []types.PhaseReconcileOption
	TeardownOptions  []types.PhaseTeardownOption
}

func (p *PhaseAdapter) GetName() string {
	return p.Phase.Name
}

func (p *PhaseAdapter) GetObjects() []client.Object {
	objects := make([]client.Object, 0, len(p.Phase.Objects))
	for _, obj := range p.Phase.Objects {
		objects = append(objects, &obj.Object)
	}
	return objects
}

func (p *PhaseAdapter) GetReconcileOptions() []types.PhaseReconcileOption {
	return p.ReconcileOptions
}

func (p *PhaseAdapter) GetTeardownOptions() []types.PhaseTeardownOption {
	return p.TeardownOptions
}

func (p *PhaseAdapter) WithReconcileOptions(opts ...types.PhaseReconcileOption) types.PhaseBuilder {
	p.ReconcileOptions = append(p.ReconcileOptions, opts...)
	return p
}

func (p *PhaseAdapter) WithTeardownOptions(opts ...types.PhaseTeardownOption) types.PhaseBuilder {
	p.TeardownOptions = append(p.TeardownOptions, opts...)
	return p
}
