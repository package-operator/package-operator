package cmdutil

import (
	"k8s.io/apimachinery/pkg/runtime"

	pkoapis "package-operator.run/apis"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

var (
	ValidateScheme = runtime.NewScheme()
)

func init() {
	if err := pkoapis.AddToScheme(ValidateScheme); err != nil {
		panic(err)
	}
}

func NewStructureLoader() *packageloader.Loader {
	return packageloader.New(ValidateScheme, packageloader.WithDefaults)
}
