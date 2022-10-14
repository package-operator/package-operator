package preflight

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
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
	ctx context.Context, owner client.Object,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error) {
	if len(owner.GetNamespace()) == 0 {
		// Owner is cluster-scoped
		// we allow objects in multiple namespaces :)
		return
	}

	if len(phase.Class) > 0 {
		// the plugin implementation has the final say, when class is set.
		return
	}

	// ObjectDeployment/ObjectSet/ObjectSetPhase
	// All objects need to be namespace-scoped and either have a namespace equal
	// to their owner or empty so it can be defaulted.
	for i, obj := range phase.Objects {
		if len(obj.Object.GetNamespace()) > 0 &&
			obj.Object.GetNamespace() != owner.GetNamespace() {
			violations = append(violations, Violation{
				Position: fmt.Sprintf("Phase %q, object No.%d", phase.Name, i),
				Error:    "Must stay within the same namespace.",
			})
		}

		gvk := obj.Object.GroupVersionKind()
		mapping, err := p.restMapper.RESTMapping(
			gvk.GroupKind(), gvk.Version)
		if meta.IsNoMatchError(err) {
			// covered by APIsExistence check
			continue
		}
		if err != nil {
			return violations, err
		}

		if mapping.Scope != meta.RESTScopeNamespace {
			violations = append(violations, Violation{
				Position: fmt.Sprintf("Phase %q, object No.%d", phase.Name, i),
				Error:    "Must be namespaced scoped when part of an non-cluster-scoped API.",
			})
		}
	}
	return
}
