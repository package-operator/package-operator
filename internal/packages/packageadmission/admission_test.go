package packageadmission_test

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := manifestsv1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := apiextensions.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
