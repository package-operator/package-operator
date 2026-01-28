package hostedclusterpackages

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// toUnstructured converts a typed HostedClustedPackage object to an unstructured.Unstructured.
// Unspecified/defaulted fields will be dropped during the conversion and are not present in the returned data.
func toUnstructured(hostedClusterPackage *corev1alpha1.HostedClusterPackage) (*unstructured.Unstructured, error) {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(hostedClusterPackage)
	if err != nil {
		return nil, fmt.Errorf("converting HostedClusterPackage to unstructured: %w", err)
	}

	uns := &unstructured.Unstructured{
		Object: m,
	}
	uns.SetGroupVersionKind(hostedClusterPackage.GroupVersionKind())

	return uns, nil
}
