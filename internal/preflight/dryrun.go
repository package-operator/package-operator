package preflight

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CreationDryRun struct {
	client client.Writer
}

func NewCreationDryRun(client client.Writer) *CreationDryRun {
	return &CreationDryRun{
		client: client,
	}
}

func (p *CreationDryRun) Check(
	ctx context.Context, _, obj client.Object,
) (violations []Violation, err error) {
	defer addPositionToViolations(ctx, obj, &violations)

	drerr := p.client.Create(ctx, obj.DeepCopyObject().(client.Object), client.DryRunAll)
	if errors.IsAlreadyExists(drerr) {
		return
	}
	if drerr != nil {
		violations = append(
			violations, Violation{
				Error: drerr.Error(),
			})
	}
	return
}
