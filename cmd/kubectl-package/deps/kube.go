package deps

import (
	"k8s.io/apimachinery/pkg/runtime"

	internalcmd "package-operator.run/package-operator/internal/cmd"
)

func ProvideScheme() (*runtime.Scheme, error) {
	return internalcmd.NewScheme()
}
