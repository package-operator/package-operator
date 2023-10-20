// Package core contains general API schema definitons.
package core

import (
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/apis/core/v1alpha1"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme.
var AddToSchemes runtime.SchemeBuilder = runtime.SchemeBuilder{
	v1alpha1.SchemeBuilder.AddToScheme,
}

// AddToScheme adds all core Resources to the Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
