package bootstrap

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/apis"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := apis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := appsv1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}
