package preflight

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type ObjectDuplicate struct{}

var _ phasesChecker = (*ObjectDuplicate)(nil)

func NewObjectDuplicate() *ObjectDuplicate {
	return &ObjectDuplicate{}
}

func (od *ObjectDuplicate) Check(
	_ context.Context, phases []corev1alpha1.ObjectSetTemplatePhase,
) (violations []Violation, err error) {
	visited := map[string]bool{}
	for _, phase := range phases {
		for _, objectSetObject := range phase.Objects {
			object := &objectSetObject.Object
			gvk := object.GroupVersionKind()
			groupKind := gvk.GroupKind().String()
			objectKey := client.ObjectKeyFromObject(object).String() // namespace and name
			key := fmt.Sprintf("%s %s", groupKind, objectKey)
			if _, ok := visited[key]; ok {
				violations = append(violations, Violation{
					Error:    "Duplicate Object",
					Position: fmt.Sprintf("Phase %q, %s", phase.Name, key),
				})
			} else {
				visited[key] = true
			}
		}
	}
	return
}
