package preflight

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Prevents namespace escalation from users specifying cluster-scoped resources or
// resources in other namespaces in non-cluster-scoped APIs.
type NamespaceEscalation struct {
	restMapper meta.RESTMapper
}

var _ checker = (*NamespaceEscalation)(nil)

func NewNamespaceEscalation(restMapper meta.RESTMapper) *NamespaceEscalation {
	return &NamespaceEscalation{
		restMapper: restMapper,
	}
}

func (p *NamespaceEscalation) Check(
	ctx context.Context, owner,
	obj client.Object,
) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, violations)

	if len(owner.GetNamespace()) == 0 {
		// Owner is cluster-scoped
		// we allow objects in multiple namespaces :)
		return
	}

	if phase, ok := phaseFromContext(ctx); ok && len(phase.Class) > 0 {
		// the plugin implementation has the final say, when class is set.
		return
	}

	// All objects need to be namespace-scoped and either have a namespace equal
	// to their owner or empty so it can be defaulted.
	if len(obj.GetNamespace()) > 0 &&
		obj.GetNamespace() != owner.GetNamespace() {
		violations = append(violations, Violation{
			Position: fmt.Sprintf("Object %s", obj.GetName()),
			Error:    "Must stay within the same namespace.",
		})
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	mapping, err := p.restMapper.RESTMapping(
		gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) {
		// covered by APIsExistence check
		return
	}
	if err != nil {
		return violations, err
	}

	if mapping.Scope != meta.RESTScopeNamespace {
		violations = append(violations, Violation{
			Error: "Must be namespaced scoped when part of an non-cluster-scoped API.",
		})
	}
	return
}
