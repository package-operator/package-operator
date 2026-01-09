package fix

import (
	"context"
	"fmt"
	"regexp"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var versionRe = regexp.MustCompile(`(?m)version:\n.*description: Object Version\.\n.*type: string\n`)

const (
	osCRDName  = "objectsets.package-operator.run"
	cosCRDName = "clusterobjectsets.package-operator.run"
)

// ControllerOfVersion adds the .status.controllerOf[].version fields to the CRD spec of ObjectSets.
// Without this fix upgrades from PKO 1.18.2 to newer versions fail.
type ControllerOfVersion struct{}

func (f ControllerOfVersion) Check(ctx context.Context, fc Context) (bool, error) {
	cosCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := fc.Client.Get(ctx, client.ObjectKey{
		Name: cosCRDName,
	}, cosCRD); err != nil {
		return false, fmt.Errorf("looking up ClusterObjectSet CRD: %w", err)
	}

	osCRD := &apiextensionsv1.CustomResourceDefinition{}
	if err := fc.Client.Get(ctx, client.ObjectKey{
		Name: osCRDName,
	}, osCRD); err != nil {
		return false, fmt.Errorf("looking up ObjectSet CRD: %w", err)
	}

	cosHasVersionField, err := f.hasControllerOfVersionField(cosCRD)
	if err != nil {
		return false, err
	}
	osHasVersionField, err := f.hasControllerOfVersionField(osCRD)
	if err != nil {
		return false, err
	}

	return !cosHasVersionField || !osHasVersionField, nil
}

func (f ControllerOfVersion) hasControllerOfVersionField(crd *apiextensionsv1.CustomResourceDefinition) (bool, error) {
	crdYAML, err := yaml.Marshal(crd)
	if err != nil {
		return false, err
	}

	if len(versionRe.FindAll(crdYAML, -1)) == 0 {
		return false, nil
	}
	return true, nil
}

func (f ControllerOfVersion) Run(ctx context.Context, fc Context) error {
	osCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: osCRDName,
		},
	}
	if err := fc.Client.Patch(ctx, osCRD, client.RawPatch(types.JSONPatchType, controllerOfVersionPatch)); err != nil {
		return fmt.Errorf("patching ObjectSets CRD: %w", err)
	}

	cosCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: cosCRDName,
		},
	}
	if err := fc.Client.Patch(ctx, cosCRD, client.RawPatch(types.JSONPatchType, controllerOfVersionPatch)); err != nil {
		return fmt.Errorf("patching ClusterObjectSets CRD: %w", err)
	}
	return nil
}

//nolint:lll
var controllerOfVersionPatch = []byte(`[
    {
        "op": "replace",
        "path": "/spec/versions/0/schema/openAPIV3Schema/properties/status/properties/revision/description",
        "value": "Deprecated: use .spec.revision instead"
    },
    {
        "op": "add",
        "path": "/spec/versions/0/schema/openAPIV3Schema/properties/status/properties/controllerOf/items/required/3",
        "value": "version"
    },
    {
        "op": "add",
        "path": "/spec/versions/0/schema/openAPIV3Schema/properties/status/properties/controllerOf/items/properties/version",
        "value": {
            "description": "Object Version.",
            "type": "string"
        }
    },
    {
        "op": "add",
        "path": "/spec/versions/0/schema/openAPIV3Schema/properties/spec/x-kubernetes-validations/4",
        "value": {
            "message": "revision is immutable",
            "rule": "!has(oldSelf.revision) || (self.revision == oldSelf.revision)"
        }
    },
    {
        "op": "add",
        "path": "/spec/versions/0/schema/openAPIV3Schema/properties/spec/properties/revision",
        "value": {
            "description": "Computed revision number, monotonically increasing.",
            "format": "int64",
            "type": "integer"
        }
    }
]`)
