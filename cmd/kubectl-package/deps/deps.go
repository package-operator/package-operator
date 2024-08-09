package deps

import (
	"go.uber.org/dig"

	"package-operator.run/cmd/kubectl-package/rootcmd"
)

func Build() (*dig.Container, error) {
	container := dig.New()

	for _, c := range constructors() {
		if err := container.Provide(c); err != nil {
			return nil, err
		}
	}

	return container, nil
}

func constructors() []any {
	return []any{
		rootcmd.ProvideRootCmd,
		ProvideIOStreams,
		ProvideArgs,
		ProvideTreeCmd,
		ProvideClusterTreeCmd,
		ProvideUpdateCmd,
		ProvideValidateCmd,
		ProvideBuildCmd,
		ProvideVersionCmd,
		ProvideLogFactory,
		ProvideScheme,
		ProvideKubeClientFactory,
		ProvideRestConfigFactory,
		ProvideUpdater,
		ProvideBuilderFactory,
		ProvideValidator,
		ProvideRendererFactory,
		ProvideRolloutCmd,
		ProvideClientFactory,
		ProvideRolloutHistoryCmd,
		ProvideKickstartCmd,
		ProvideKickstarter,
		ProvideRolloutRollbackCmd,
	}
}
