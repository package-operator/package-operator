package cmdutil

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	pkoapis "package-operator.run/apis"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

var Scheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := manifestsv1alpha1.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := apiextensions.AddToScheme(Scheme); err != nil {
		panic(err)
	}
}

func NewStructureLoader() *packageloader.Loader {
	return packageloader.New(Scheme, packageloader.WithDefaults)
}
