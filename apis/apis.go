// Package apis contains all API schema definitons used by package operator.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/apis/core"
	"package-operator.run/apis/manifests"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme.
var AddToSchemes runtime.SchemeBuilder = runtime.SchemeBuilder{
	core.AddToScheme,
	manifests.AddToScheme,
}

// AddToScheme adds all manifests Resources to the Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
