package bootstrap

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	pkoapis "package-operator.run/apis"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := appsv1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
