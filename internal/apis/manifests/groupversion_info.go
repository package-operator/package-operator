// +kubebuilder:object:generate=true
package manifests

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "manifests.package-operator.run", Version: runtime.APIVersionInternal}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder runtime.SchemeBuilder

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func register(objs ...runtime.Object) {
	SchemeBuilder.Register(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(GroupVersion, objs...)
		// scheme.AddConversionFunc()
		return nil
	})
}
