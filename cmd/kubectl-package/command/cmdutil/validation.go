package cmdutil

import (
	pkoapis "package-operator.run/apis"

	"k8s.io/apimachinery/pkg/runtime"
	"package-operator.run/package-operator/internal/packages/packagestructure"
)

var (
	ValidateScheme = runtime.NewScheme()
)

func init() {
	if err := pkoapis.AddToScheme(ValidateScheme); err != nil {
		panic(err)
	}
}

func NewStructureLoader() *packagestructure.Loader {
	structureLoaderOpts := []packagestructure.LoaderOption{
		packagestructure.WithManifestValidators(
			packagestructure.DefaultValidators,
		),
	}

	return packagestructure.NewLoader(ValidateScheme, structureLoaderOpts...)
}
