package parametrize

import (
	"encoding/json"
	"fmt"

	"github.com/joeycumines/go-dotnotation/dotnotation"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

type DeploymentOptions struct {
	Replicas      bool
	Tolerations   bool
	NodeSelectors bool
	Resources     bool
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
		instructions = append(instructions, Pipeline(replicasAccess, "spec.replicas"))
	}

	if opts.Tolerations {
		configSchema.Properties["tolerations"] = apiextensionsv1.JSONSchemaProps{
			Type:        "array",
			Description: fmt.Sprintf("Additional tolerations for Deployment %s/%s.", obj.GetNamespace(), obj.GetName()),
			Default: &apiextensionsv1.JSON{
				Raw: []byte("[]"),
			},
			Items: &apiextensionsv1.JSONSchemaPropsOrArray{
				Schema: &apiextensionsv1.JSONSchemaProps{
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"effect": {
							Type: "string",
						},
						"key": {
							Type: "string",
						},
						"operator": {
							Type: "string",
						},
						"tolerationSeconds": {
							Format: "int64",
							Type:   "integer",
						},
						"value": {
							Type: "string",
						},
					},
					Type: "object",
				},
			},
		}

		_, err := dotnotation.Get(obj.Object, "spec.template.spec.tolerations")
		if err != nil {
			if err := dotnotation.Set(obj.Object, "spec.template.spec.tolerations", []interface{}{}); err != nil {
				return nil, err
			}
		}

		tolerationsAccess := fmt.Sprintf(
			`index .config "deployments" %q %q "tolerations"`,
			obj.GetNamespace(), obj.GetName())
		instructions = append(instructions, MergeBlock(tolerationsAccess, "spec.template.spec.tolerations"))
	}

	if opts.NodeSelectors {
		configSchema.Properties["nodeSelector"] = apiextensionsv1.JSONSchemaProps{
			Type:                   "object",
			Description:            fmt.Sprintf("NodeSelector for Deployment %s/%s.", obj.GetNamespace(), obj.GetName()),
			XPreserveUnknownFields: ptr.To(true),
		}

		nodeSelectorAccess := fmt.Sprintf(
			`index .config "deployments" %q %q "nodeSelector" | toJson`,
			obj.GetNamespace(), obj.GetName())
		instructions = append(instructions, Pipeline(nodeSelectorAccess, "spec.template.spec.nodeSelector"))
	}

	if opts.Resources {
		configSchema.Properties["containers"] = apiextensionsv1.JSONSchemaProps{
			Type: "object",
			Default: &apiextensionsv1.JSON{
				Raw: []byte("{}"),
			},
			Properties: map[string]apiextensionsv1.JSONSchemaProps{},
		}

		containers, err := dotnotation.Get(obj.Object, "spec.template.spec.containers")
		if err != nil {
			return nil, err
		}
		for i, container := range containers.([]interface{}) {
			c := container.(map[string]interface{})
			name, _, err := unstructured.NestedString(c, "name")
			if err != nil {
				return nil, err
			}

			originalResources, _, err := unstructured.NestedMap(c, "resources")
			if err != nil {
				return nil, err
			}
			if originalResources == nil {
				originalResources = map[string]interface{}{}
			}
			defaultRaw, err := json.Marshal(originalResources)
			if err != nil {
				return nil, err
			}

			typeResource := apiextensionsv1.JSONSchemaProps{
				Type: "object",
				AdditionalProperties: &apiextensionsv1.JSONSchemaPropsOrBool{
					Schema: &apiextensionsv1.JSONSchemaProps{
						AnyOf: []apiextensionsv1.JSONSchemaProps{
							{Type: "integer"},
							{Type: "string"},
						},
						Pattern:      `^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$`,
						XIntOrString: true,
					},
				},
			}

			configSchema.Properties["containers"].Properties[name] = apiextensionsv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"resources": {
						Type: "object",
						Default: &apiextensionsv1.JSON{
							Raw: defaultRaw,
						},
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"limits":   typeResource,
							"requests": typeResource,
						},
					},
				},
				Default: &apiextensionsv1.JSON{
					Raw: []byte("{}"),
				},
			}

			nodeSelectorAccess := fmt.Sprintf(
				`index .config "deployments" %q %q "containers" %q "resources" | toJson`,
				obj.GetNamespace(), obj.GetName(), name)
			instructions = append(instructions, Pipeline(nodeSelectorAccess, fmt.Sprintf("spec.template.spec.containers.%d.resources", i)))
		}
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
