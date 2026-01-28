package hostedclusterpackages

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "k8s.io/client-go/applyconfigurations/meta/v1"

	corev1alpha1acs "package-operator.run/apis/applyconfigurations/core/v1alpha1"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// ExtractPackageTemplateFields extracts specified fields from the given HostedClusterPackage's .spec.template
// by remarshaling them into an applyconfiguration.
// This ensures that only user-specified fields get sent to the API and no go defaulting
// on unspecified fields takes place.
// Defaulting for example always send intents with .spec.paused set to false,
// which prevents users from overriding the field with their own intents.
func ExtractPackageTemplateFields(
	hcpkg *unstructured.Unstructured,
) (*corev1alpha1acs.PackageTemplateSpecApplyConfiguration, error) {
	ac := &corev1alpha1acs.PackageTemplateSpecApplyConfiguration{
		ObjectMetaApplyConfiguration: &v1.ObjectMetaApplyConfiguration{},
		Spec:                         &corev1alpha1acs.PackageSpecApplyConfiguration{},
	}

	meta, ok, err := unstructured.NestedMap(hcpkg.Object, "spec", "template", "metadata")
	if err != nil {
		return nil, fmt.Errorf("extracting template metadata: %w", err)
	}
	if !ok {
		meta = map[string]any{}
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshaling meta bytes: %w", err)
	}

	if err := json.Unmarshal(metaBytes, ac.ObjectMetaApplyConfiguration); err != nil {
		return nil, fmt.Errorf("unmarshaling meta: %w", err)
	}

	spec, ok, err := unstructured.NestedMap(hcpkg.Object, "spec", "template", "spec")
	if err != nil {
		return nil, fmt.Errorf("extracting template spec: %w", err)
	}
	if !ok {
		spec = map[string]any{}
	}

	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling spec: %w", err)
	}

	if err := json.Unmarshal(specBytes, ac.Spec); err != nil {
		return nil, fmt.Errorf("unmarshaling spec: %w", err)
	}

	return ac, nil
}
