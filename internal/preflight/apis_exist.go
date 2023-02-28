package preflight

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
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

func (p *APIExistence) Check(
	ctx context.Context, owner client.Object,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error) {
	for i, obj := range phase.Objects {
		gvk := obj.Object.GroupVersionKind()
		_, err := p.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if meta.IsNoMatchError(err) {
			violations = append(violations, Violation{
				Position: fmt.Sprintf("Phase %q, object No.%d", phase.Name, i),
				Error:    fmt.Sprintf("%s not registered on the api server.", gvk),
			})
			continue
		}
		if err != nil {
			return violations, err
		}
	}

	return
}

func (p *APIExistence) CheckObj(
	ctx context.Context, owner,
	obj client.Object,
) (violations []Violation, err error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	_, err = p.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) {
		violations = append(violations, Violation{
			Position: fmt.Sprintf("object %s", obj.GetName()),
			Error:    fmt.Sprintf("%s not registered on the api server.", gvk),
		})
	}
	if err != nil {
		return
	}
	return
}
