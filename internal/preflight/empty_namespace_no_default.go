package preflight

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (p *EmptyNamespaceNoDefault) Check(
	ctx context.Context, owner,
	obj client.Object,
) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, &violations)

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
