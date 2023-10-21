package adapters

import (
	"k8s.io/apimachinery/pkg/runtime"

	apis "package-operator.run/apis"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := apis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
