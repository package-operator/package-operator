package packages

import "package-operator.run/internal/packages/internal/packagetypes"

type (
	// Interface implemented by all registry options.
	RegistryOption = packagetypes.RegistryOption
	// Insecure is an Option that allows image references to be fetched without TLS.
	WithInsecure = packagetypes.WithInsecure
)
