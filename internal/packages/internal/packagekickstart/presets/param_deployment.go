package presets

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joeycumines/go-dotnotation/dotnotation"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"package-operator.run/internal/packages/internal/packagekickstart/parametrize"
)

type DeploymentOptions struct {
	Replicas      bool
	Tolerations   bool
	NodeSelectors bool
	Images        bool
	Resources     bool
	Env           bool
	GenericOptions
}

func Deployment(
	obj unstructured.Unstructured,
	schema *apiextensionsv1.JSONSchemaProps,
	imageContainer *ImageContainer,
	opts DeploymentOptions,
) (
	[]byte, error,
) {
	var instructions []parametrize.Instruction
	if opts.Namespaces {
		if inst, ok := parametrizeNamespace(obj); ok {
			instructions = append(instructions, inst...)
		}
	}

	configSchema := apiextensionsv1.JSONSchemaProps{
		Type:       "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
		Default: &apiextensionsv1.JSON{
			Raw: []byte("{}"),
		},
	}

	// Param options.
	if opts.Replicas {
		instructions = append(instructions,
			parametrizeDeploymentReplicas(obj, &configSchema)...)
	}
	if opts.NodeSelectors {
		instructions = append(instructions,
			parametrizeDeploymentNodeSelector(obj, &configSchema)...)
	}
	if opts.Images {
		i, err := parametrizeDeploymentImages(obj, &configSchema, imageContainer)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, i...)
	}
	if opts.Tolerations {
		i, err := parametrizeDeploymentTolerations(obj, &configSchema)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, i...)
	}
	if opts.Env || opts.Resources {
		i, err := parametrizeDeploymentContainers(obj, &configSchema, opts)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, i...)
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

	out, err := parametrize.Execute(obj, instructions...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func parametrizeDeploymentTolerations(
	obj unstructured.Unstructured,
	configSchema *apiextensionsv1.JSONSchemaProps,
) ([]parametrize.Instruction, error) {
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
		if err := dotnotation.Set(obj.Object, "spec.template.spec.tolerations", []any{}); err != nil {
			return nil, err
		}
	}

	tolerationsAccess := fmt.Sprintf(
		`index .config "deployments" %q %q "tolerations"`,
		obj.GetNamespace(), obj.GetName())
	return []parametrize.Instruction{
		parametrize.MergeBlock(tolerationsAccess, "spec.template.spec.tolerations"),
	}, nil
}

func parametrizeDeploymentImages(
	obj unstructured.Unstructured,
	configSchema *apiextensionsv1.JSONSchemaProps,
	imageContainer *ImageContainer,
) ([]parametrize.Instruction, error) {
	//nolint:prealloc
	var instructions []parametrize.Instruction
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
	for i, container := range containers.([]any) {
		c := container.(map[string]any)
		name, _, err := unstructured.NestedString(c, "name")
		if err != nil {
			return nil, err
		}

		imageDotNotation := fmt.Sprintf("spec.template.spec.containers.%d.image", i)
		imageI, err := dotnotation.Get(obj.Object, imageDotNotation)
		if err != nil {
			return nil, err
		}
		image := imageI.(string)
		if strings.Contains(image, "@") {
			// Skip images that are already referenced by digest.
			continue
		}
		imageName := imageContainer.Add(name, image)
		imageAccess := fmt.Sprintf(`index .images %q`, imageName)
		instructions = append(instructions, parametrize.Pipeline(imageAccess, imageDotNotation))
	}
	return instructions, nil
}

func parametrizeDeploymentNodeSelector(
	obj unstructured.Unstructured,
	configSchema *apiextensionsv1.JSONSchemaProps,
) []parametrize.Instruction {
	configSchema.Properties["nodeSelector"] = apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Description: fmt.Sprintf(
			"NodeSelector for Deployment %s/%s.",
			obj.GetNamespace(), obj.GetName()),
		XPreserveUnknownFields: ptr.To(true),
	}

	nodeSelectorAccess := fmt.Sprintf(
		`index .config "deployments" %q %q "nodeSelector" | toJson`,
		obj.GetNamespace(), obj.GetName())
	return []parametrize.Instruction{
		parametrize.Pipeline(nodeSelectorAccess, "spec.template.spec.nodeSelector"),
	}
}

func parametrizeDeploymentReplicas(
	obj unstructured.Unstructured,
	configSchema *apiextensionsv1.JSONSchemaProps,
) []parametrize.Instruction {
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
	return []parametrize.Instruction{
		parametrize.Pipeline(replicasAccess, "spec.replicas"),
	}
}

func parametrizeDeploymentContainers(
	obj unstructured.Unstructured,
	configSchema *apiextensionsv1.JSONSchemaProps,
	opts DeploymentOptions,
) ([]parametrize.Instruction, error) {
	var instructions []parametrize.Instruction
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
	for i, container := range containers.([]any) {
		c := container.(map[string]any)
		name, _, err := unstructured.NestedString(c, "name")
		if err != nil {
			return nil, err
		}
		configSchema.Properties["containers"].Properties[name] = apiextensionsv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]apiextensionsv1.JSONSchemaProps{},
			Default: &apiextensionsv1.JSON{
				Raw: []byte("{}"),
			},
		}

		if opts.Env {
			configSchema.Properties["containers"].
				Properties[name].
				Properties["env"] = apiextensionsv1.JSONSchemaProps{
				Type: "array",
				Default: &apiextensionsv1.JSON{
					Raw: []byte("[]"),
				},
				Items: &apiextensionsv1.JSONSchemaPropsOrArray{
					Schema: &apiextensionsv1.JSONSchemaProps{
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"name": {
								Type: "string",
							},
							"value": {
								Type: "string",
							},
							"valueFrom": {
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"configMapKeyRef": {
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"key": {
												Type: "string",
											},
											"name": {
												Type: "string",
											},
											"optional": {
												Type: "boolean",
											},
										},
										Required: []string{"key"},
										Type:     "object",
									},
									"fieldRef": {
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"apiVersion": {
												Type: "string",
											},
											"fieldPath": {
												Type: "string",
											},
										},
										Required: []string{"fieldPath"},
										Type:     "object",
									},
									// TODO: Schema is not accepted.
									// "resourceFieldRef": {
									// 	Properties: map[string]apiextensionsv1.JSONSchemaProps{
									// 		"containerName": {
									// 			Type: "string",
									// 		},
									// 		"divisor": {
									// 			OneOf: []apiextensionsv1.JSONSchemaProps{
									// 				{Type: "string"},
									// 				{Type: "number"},
									// 			},
									// 		},
									// 		"resource": {
									// 			Type: "string",
									// 		},
									// 	},
									// 	Required: []string{"resource"},
									// 	Type:     "object",
									// },
									"secretKeyRef": {
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"key":      {Type: "string"},
											"name":     {Type: "string"},
											"optional": {Type: "boolean"},
										},
										Required: []string{"key"},
										Type:     "object",
									},
								},
								Type: "object",
							},
						},
						Type: "object",
					},
				},
			}

			envDotNotation := fmt.Sprintf("spec.template.spec.containers.%d.env", i)
			_, err := dotnotation.Get(obj.Object, envDotNotation)
			if err != nil {
				if err := dotnotation.Set(obj.Object, envDotNotation, []any{}); err != nil {
					return nil, err
				}
			}

			tolerationsAccess := fmt.Sprintf(
				`index .config "deployments" %q %q "containers" %q "env"`,
				obj.GetNamespace(), obj.GetName(), name)
			instructions = append(instructions, parametrize.MergeBlock(tolerationsAccess, envDotNotation))
		}

		if opts.Resources {
			originalResources, _, err := unstructured.NestedMap(c, "resources")
			if err != nil {
				return nil, err
			}
			if originalResources == nil {
				originalResources = map[string]any{}
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
						Pattern: `^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))` +
							`(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)` +
							`|(\.[0-9]+))))?$`,
						XIntOrString: true,
					},
				},
			}

			configSchema.Properties["containers"].
				Properties[name].
				Properties["resources"] = apiextensionsv1.JSONSchemaProps{
				Type: "object",
				Default: &apiextensionsv1.JSON{
					Raw: defaultRaw,
				},
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"limits":   typeResource,
					"requests": typeResource,
				},
			}

			nodeSelectorAccess := fmt.Sprintf(
				`index .config "deployments" %q %q "containers" %q "resources" | toJson`,
				obj.GetNamespace(), obj.GetName(), name)
			instructions = append(instructions,
				parametrize.Pipeline(nodeSelectorAccess,
					fmt.Sprintf("spec.template.spec.containers.%d.resources", i)))
		}
	}
	return instructions, nil
}
