package packagedeploy

import (
	"k8s.io/apimachinery/pkg/runtime"

	pkoapis "package-operator.run/apis"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
