package parametrize

import (
	"fmt"

	"github.com/joeycumines/go-dotnotation/dotnotation"
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
		Default: &apiextensionsv1.JSON{
			Raw: []byte("{}"),
		},
	}

	if opts.Replicas {
		originalValue, err := dotnotation.Get(obj.Object, "spec.replicas")
		if err != nil {
			originalValue = 1
		}
		configSchema.Properties["replicas"] = apiextensionsv1.JSONSchemaProps{
			Type:        "integer",
			Format:      "int32",
			Description: fmt.Sprintf("Replica count for Deployment %s/%s.", obj.GetNamespace(), obj.GetName()),
			Default: &apiextensionsv1.JSON{
				Raw: []byte(fmt.Sprintf("%v", originalValue)),
			},
		}
		// access via index function because namespaces and names may have dashes in them.
		replicasAccess := fmt.Sprintf(
			`index .config "deployments" %q %q "replicas"`,
			obj.GetNamespace(), obj.GetName())
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
			Default: &apiextensionsv1.JSON{
				Raw: []byte("{}"),
			},
		}
	}
	if _, ok := schema.Properties["deployments"].
		Properties[obj.GetNamespace()]; !ok {
		schema.Properties["deployments"].
			Properties[obj.GetNamespace()] = apiextensionsv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{},
			Default: &apiextensionsv1.JSON{
				Raw: []byte("{}"),
			},
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
