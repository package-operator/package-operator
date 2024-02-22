package packaging

import "package-operator.run/internal/packages"

type (
	// RegistryOption is implemented by all registry options.
	RegistryOption = packages.RegistryOption
	// WithInsecure allows image references to be fetched without TLS.
	WithInsecure = packages.WithInsecure
)
