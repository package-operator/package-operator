// The package v1alpha1 contains API Schema definitions for the v1alpha1 version of the coordination Package Operator API group,
// containing helper APIs to coordinate rollout into the cluster.
// +kubebuilder:object:generate=true
// +groupName=coordination.package-operator.run
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{
		Group:   "coordination.package-operator.run",
		Version: "v1alpha1",
	}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
