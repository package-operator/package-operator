package addons

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder = runtime.SchemeBuilder{
	v1alpha1.SchemeBuilder.AddToScheme,
}

// AddToScheme adds all addon Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
