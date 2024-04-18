package preflight

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Prevents the use of APIs not registered into the kube-apiserver.
type NoOwnerReferences struct {
	restMapper meta.RESTMapper
}

var _ checker = (*NoOwnerReferences)(nil)

func NewNoOwnerReferences(restMapper meta.RESTMapper) *NoOwnerReferences {
	return &NoOwnerReferences{
		restMapper: restMapper,
	}
}

func (p *NoOwnerReferences) Check(ctx context.Context, _, obj client.Object) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, &violations)

	if len(obj.GetOwnerReferences()) != 0 {
		violations = append(violations, Violation{
			Error: "Object must not have a owner reference.",
		})
	}

	return
}
