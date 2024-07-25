package presets

import (
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ParametrizeOptions struct {
	// Parametrize .metadata.namespace on all objects.
	// .config.namespace will set the namespace on all objects and
	// .config.namespaces.<name> can be used to override the name of a specifig namespace.
	Namespaces bool

	// Replicas adds a variable to set Deployment replicas.
	// .config.deployments.<namespace>.<name>.replicas is setting replicas.
	Replicas      bool
	Tolerations   bool
	NodeSelectors bool
	Resources     bool
	Env           bool
	Images        bool
}

const (
	namespaceParam     = "namespaces"
	replicasParam      = "replicas"
	tolerationsParam   = "tolerations"
	nodeSelectorsParam = "nodeselectors"
	resourcesParam     = "resources"
	envParam           = "env"
	imagesParam        = "images"
	allParam           = "all"
)

func ParametrizeOptionsFromFlags(opts []string) ParametrizeOptions {
	if slices.Contains(opts, "all") {
		return ParametrizeOptions{
			Namespaces:    true,
			Replicas:      true,
			Tolerations:   true,
			NodeSelectors: true,
			Resources:     true,
			Env:           true,
			Images:        true,
		}
	}

	return ParametrizeOptions{
		Namespaces:    slices.Contains(opts, namespaceParam),
		Replicas:      slices.Contains(opts, replicasParam),
		Tolerations:   slices.Contains(opts, tolerationsParam),
		NodeSelectors: slices.Contains(opts, nodeSelectorsParam),
		Resources:     slices.Contains(opts, resourcesParam),
		Env:           slices.Contains(opts, envParam),
		Images:        slices.Contains(opts, imagesParam),
	}
}

func (opts *ParametrizeOptions) IsEmpty() bool {
	return *opts == ParametrizeOptions{}
}

func Parametrize(
	obj unstructured.Unstructured,
	scheme *apiextensionsv1.JSONSchemaProps,
	imageContainer *ImageContainer,
	opts ParametrizeOptions,
) ([]byte, bool, error) {
	if opts.IsEmpty() {
		return nil, false, nil
	}

	if obj.GroupVersionKind() == deployGVK {
		out, err := Deployment(obj, scheme, imageContainer, DeploymentOptions{
			Replicas:      opts.Replicas,
			Tolerations:   opts.Tolerations,
			NodeSelectors: opts.NodeSelectors,
			Resources:     opts.Resources,
			Env:           opts.Env,
			Images:        opts.Images,
			GenericOptions: GenericOptions{
				Namespaces: opts.Namespaces,
			},
		})
		if err != nil {
			return nil, false, err
		}
		return out, true, nil
	}

	return Generic(obj, GenericOptions{
		Namespaces: opts.Namespaces,
	})
}
