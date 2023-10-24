package packagestructure

import (
	"k8s.io/apimachinery/pkg/runtime"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/apis/manifests"
)

var scheme = runtime.NewScheme()

func init() {
	b := runtime.SchemeBuilder{
		manifests.AddToScheme,
		manifestsv1alpha1.AddToScheme,
	}
	if err := b.AddToScheme(scheme); err != nil {
		panic(err)
	}
}

// DefaultStructuralLoader instance with the scheme pre-loaded.
var DefaultStructuralLoader = NewStructuralLoader(scheme)
