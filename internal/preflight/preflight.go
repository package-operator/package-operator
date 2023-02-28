// package preflight implements preflight checks for PKO APIs.
package preflight

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type Violation struct {
	// Position the violation was found.
	Position string
	// Error describing the violation.
	Error string
}

func (v *Violation) String() string {
	return fmt.Sprintf("%s: %s", v.Position, v.Error)
}

type checker interface {
	Check(
		ctx context.Context, owner client.Object,
		phase corev1alpha1.ObjectSetTemplatePhase,
	) (violations []Violation, err error)
	CheckObj(
		ctx context.Context, owner,
		obj client.Object,
	) (violations []Violation, err error)
}

// Runs a list of preflight checks and aggregates the result into a single list of violations.
type List []checker

func (l List) Check(
	ctx context.Context, owner client.Object,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error) {
	for _, checker := range l {
		v, err := checker.Check(ctx, owner, phase)
		if err != nil {
			return violations, err
		}
		violations = append(violations, v...)
	}
	return
}

func (l List) CheckObj(
	ctx context.Context, owner,
	obj client.Object,
) (violations []Violation, err error) {
	for _, checker := range l {
		v, err := checker.CheckObj(ctx, owner, obj)
		if err != nil {
			return violations, err
		}
		violations = append(violations, v...)
	}
	return
}
