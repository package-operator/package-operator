// package preflight implements preflight checks for PKO APIs.
package preflight

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type Error struct {
	Violations []Violation
}

func (e *Error) Error() string {
	vs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		vs[i] = v.String()
	}

	return strings.Join(vs, ", ")
}

type Violation struct {
	// Position the violation was found.
	Position string
	// Error describing the violation.
	Error string
}

func (v *Violation) String() string {
	return fmt.Sprintf("%s: %s", v.Position, v.Error)
}

type contextKey string

const phaseContextKey contextKey = "_phase"

func NewContextWithPhase(ctx context.Context, phase corev1alpha1.ObjectSetTemplatePhase) context.Context {
	return context.WithValue(ctx, phaseContextKey, phase)
}

func phaseFromContext(ctx context.Context) (
	phase corev1alpha1.ObjectSetTemplatePhase, found bool,
) {
	phaseI := ctx.Value(phaseContextKey)
	if phaseI == nil {
		return
	}
	return phaseI.(corev1alpha1.ObjectSetTemplatePhase), true
}

func addPositionToViolations(
	ctx context.Context, obj client.Object, vs *[]Violation,
) {
	objPosition := fmt.Sprintf("%s %s",
		obj.GetObjectKind().GroupVersionKind().Kind,
		client.ObjectKeyFromObject(obj))

	phase, ok := phaseFromContext(ctx)
	if ok {
		objPosition = fmt.Sprintf("Phase %q, %s", phase.Name, objPosition)
	}

	for i := range *vs {
		(*vs)[i].Position = objPosition
	}
}

type checker interface { //nolint: iface
	Check(
		ctx context.Context, owner, obj client.Object,
	) (violations []Violation, err error)
}

type CheckerFn func(
	ctx context.Context, owner, obj client.Object,
) (violations []Violation, err error)

func (fn CheckerFn) Check(
	ctx context.Context, owner,
	obj client.Object,
) (violations []Violation, err error) {
	return fn(ctx, owner, obj)
}

// Runs a list of preflight checks and aggregates the result into a single list of violations.
type List []checker

func (l List) Check(
	ctx context.Context, owner,
	obj client.Object,
) (violations []Violation, err error) {
	for _, checker := range l {
		v, err := checker.Check(ctx, owner, obj)
		if err != nil {
			return violations, err
		}
		violations = append(violations, v...)
	}
	return
}

func CheckAll(
	ctx context.Context, checker checker,
	owner client.Object, objs []client.Object,
) (violations []Violation, err error) {
	for _, obj := range objs {
		vs, err := checker.Check(ctx, owner, obj.DeepCopyObject().(client.Object))
		if err != nil {
			return nil, err
		}
		violations = append(violations, vs...)
	}
	return
}

func CheckAllInPhase(
	ctx context.Context, checker checker,
	owner client.Object,
	phase corev1alpha1.ObjectSetTemplatePhase,
	objs []unstructured.Unstructured,
) (violations []Violation, err error) {
	ctx = NewContextWithPhase(ctx, phase)
	for i := range phase.Objects {
		vs, err := checker.Check(ctx, owner, objs[i].DeepCopy())
		if err != nil {
			return nil, err
		}
		violations = append(violations, vs...)
	}
	return
}

type phasesChecker interface {
	Check(
		ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase,
	) (violations []Violation, err error)
}

type phasesCheckerFn func(
	ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error)

func (fn phasesCheckerFn) Check(
	ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error) {
	return fn(ctx, phases)
}

var _ phasesChecker = PhasesCheckerList(nil)

type PhasesCheckerList []phasesChecker

func (l PhasesCheckerList) Check(
	ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error) {
	for _, phasesChecker := range l {
		v, err := phasesChecker.Check(ctx, phases)
		if err != nil {
			return violations, err
		}
		violations = append(violations, v...)
	}
	return
}
