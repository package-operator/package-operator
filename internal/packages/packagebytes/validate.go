package packagebytes

import (
	"context"

	"package-operator.run/package-operator/internal/packages"
)

type Validator interface {
	Validate(ctx context.Context, fileMap FileMap) error
}

var _ Validator = (ValidatorList)(nil)

// Runs a list of Validator over the given content.
type ValidatorList []Validator

func (l ValidatorList) Validate(ctx context.Context, fileMap FileMap) error {
	var errors []error
	for _, t := range l {
		if err := t.Validate(ctx, fileMap); err != nil {
			errors = append(errors, err)
		}
	}
	return packages.NewInvalidAggregate(errors...)
}
