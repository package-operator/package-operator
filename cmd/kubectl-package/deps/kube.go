package deps

import (
	"k8s.io/apimachinery/pkg/runtime"

	internalcmd "package-operator.run/internal/cmd"
)

func ProvideScheme() (*runtime.Scheme, error) {
	return internalcmd.NewScheme()
}

func ProvideKubeClientFactory(
	cfgFactory internalcmd.RestConfigFactory, scheme *runtime.Scheme,
) internalcmd.KubeClientFactory {
	return internalcmd.NewDefaultKubeClientFactory(scheme, cfgFactory)
}

func ProvideRestConfigFactory() internalcmd.RestConfigFactory {
	return internalcmd.NewDefaultRestConfigFactory()
}
