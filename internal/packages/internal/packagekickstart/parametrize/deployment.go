package parametrize

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DeploymentOptions struct {
	Replicas bool
}

func Deployment(
	obj unstructured.Unstructured,
	schema *apiextensionsv1.JSONSchemaProps,
	opts DeploymentOptions,
) (
	[]byte, error,
) {
	var (
		instructions []Instruction
	)
	configSchema := apiextensionsv1.JSONSchemaProps{
		Type:       "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
	}

	// configKey := fmt.Sprintf(".config.deployments.%s.%s", obj.GetNamespace(), obj.GetName())
	if opts.Replicas {
		configSchema.Properties["replicas"] = apiextensionsv1.JSONSchemaProps{
			Type:        "integer",
			Format:      "int32",
			Description: fmt.Sprintf("Replica count for Deployment %s/%s.", obj.GetNamespace(), obj.GetName()),
		}
		// access via index function because namespaces and names may have dashes in them.
		replicasAccess := fmt.Sprintf(`index .config.deployments %q %q "replicas"`, obj.GetNamespace(), obj.GetName())
		instructions = append(instructions, Expression(replicasAccess, "spec.replicas"))
	}

	// Add to Schema.
	if schema.Properties == nil {
		schema.Properties = map[string]apiextensionsv1.JSONSchemaProps{}
	}
	if _, ok := schema.Properties["deployments"]; !ok {
		schema.Properties["deployments"] = apiextensionsv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{},
		}
	}
	if _, ok := schema.Properties["deployments"].
		Properties[obj.GetNamespace()]; !ok {
		schema.Properties["deployments"].
			Properties[obj.GetNamespace()] = apiextensionsv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{},
		}
	}
	schema.Properties["deployments"].
		Properties[obj.GetNamespace()].
		Properties[obj.GetName()] = configSchema

	out, err := Execute(obj, instructions...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
