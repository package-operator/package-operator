package preflight

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// Prevents the use of APIs not registered into the kube-apiserver.
type EmptyNamespaceNoDefault struct {
	restMapper meta.RESTMapper
}

var _ checker = (*EmptyNamespaceNoDefault)(nil)

func NewEmptyNamespaceNoDefault(restMapper meta.RESTMapper) *EmptyNamespaceNoDefault {
	return &EmptyNamespaceNoDefault{
		restMapper: restMapper,
	}
}

func (p *EmptyNamespaceNoDefault) CheckPhase(
	_ context.Context, owner client.Object,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error) {
	if len(owner.GetNamespace()) > 0 {
		// Owner has namespace which is the default
		return
	}

	for i, obj := range phase.Objects {
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

		if mapping.Scope == meta.RESTScopeNamespace && len(obj.Object.GetNamespace()) == 0 {
			violations = append(violations, Violation{
				Position: fmt.Sprintf("Phase %q, object No.%d", phase.Name, i),
				Error:    "Object doesn't have a namepsace and no default is provided.",
			})
		}
	}

	return
}

func (p *EmptyNamespaceNoDefault) Check(
	ctx context.Context, owner,
	obj client.Object,
) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, violations)

	if len(owner.GetNamespace()) > 0 {
		// Owner has namespace which is the default
		return
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

	if mapping.Scope == meta.RESTScopeNamespace && len(obj.GetNamespace()) == 0 {
		violations = append(violations, Violation{
			Error: "Object doesn't have a namepsace and no default is provided.",
		})
	}

	return
}
