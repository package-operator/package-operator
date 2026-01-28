package presets

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"package-operator.run/internal/packages/internal/packagekickstart/parametrize"
)

type GenericOptions struct {
	Namespaces bool
}

// Add Preset Parametrization to any objects without special handling.
func Generic(
	obj unstructured.Unstructured,
	opts GenericOptions,
) (
	[]byte, bool, error,
) {
	var instructions []parametrize.Instruction
	if opts.Namespaces {
		if inst, ok := parametrizeNamespace(obj); ok {
			instructions = append(instructions, inst...)
		}
	}

	if len(instructions) == 0 {
		return nil, false, nil
	}

	out, err := parametrize.Execute(obj, instructions...)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func parametrizeNamespace(obj unstructured.Unstructured) (
	[]parametrize.Instruction, bool,
) {
	clusterRoleBindingGK := schema.GroupKind{
		Kind:  "ClusterRoleBinding",
		Group: "rbac.authorization.k8s.io",
	}

	var instructions []parametrize.Instruction
	if obj.GroupVersionKind().GroupKind() == clusterRoleBindingGK {
		subjects, _, _ := unstructured.NestedSlice(obj.Object, "subjects")
		for i, subjectI := range subjects {
			subject := subjectI.(map[string]any)
			ns, ok := subject["namespace"].(string)
			if !ok {
				continue
			}
			p := ".config.namespace"
			if len(ns) != 0 {
				p = fmt.Sprintf("default (index .config.namespaces %q) .config.namespace", ns)
			}

			instructions = append(instructions, parametrize.Pipeline(
				p, fmt.Sprintf("subjects.%d.namespace", i)))
		}

		return instructions, true
	}

	if isClusterScoped(obj) {
		return nil, false
	}

	ns := obj.GetNamespace()
	p := ".config.namespace"
	if len(ns) != 0 {
		p = fmt.Sprintf("default (index .config.namespaces %q) .config.namespace", ns)
	}
	instructions = append(instructions, parametrize.Pipeline(p, "metadata.namespace"))
	return instructions, true
}
