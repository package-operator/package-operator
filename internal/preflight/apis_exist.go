package preflight

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Prevents the use of APIs not registered into the kube-apiserver.
type APIExistence struct {
	restMapper meta.RESTMapper
}

var _ checker = (*APIExistence)(nil)

func NewAPIExistence(restMapper meta.RESTMapper) *APIExistence {
	return &APIExistence{
		restMapper: restMapper,
	}
}

func (p *APIExistence) Check(ctx context.Context, _, obj client.Object) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, &violations)

	gvk := obj.GetObjectKind().GroupVersionKind()
	_, err = p.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) {
		violations = append(violations, Violation{
			Error: fmt.Sprintf("%s not registered on the api server.", gvk),
		})
	}
	if err != nil {
		return
	}
	return
}
