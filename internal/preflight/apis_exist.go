package preflight

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Prevents the use of APIs not registered into the kube-apiserver.
type APIExistence struct {
	restMapper restMapper
	sub        preflightChecker
}

type preflightChecker interface { //nolint: iface
	Check(ctx context.Context, owner, obj client.Object) (violations []Violation, err error)
}

type restMapper interface {
	RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error)
}

var _ checker = (*APIExistence)(nil)

func NewAPIExistence(restMapper restMapper, sub preflightChecker) *APIExistence {
	return &APIExistence{restMapper, sub}
}

func (p *APIExistence) Check(ctx context.Context, owner, obj client.Object) ([]Violation, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	_, err := p.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	switch {
	case err == nil:
		return p.sub.Check(ctx, owner, obj)
	case meta.IsNoMatchError(err):
		violations := []Violation{{Error: fmt.Sprintf("%s not registered on the api server.", gvk)}}
		addPositionToViolations(ctx, obj, &violations)

		return violations, nil
	default:
		return nil, err
	}
}
