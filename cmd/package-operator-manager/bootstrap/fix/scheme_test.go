package fix

import (
	v1apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	pkoapis "package-operator.run/apis"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := v1apiextensions.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
