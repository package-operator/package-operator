package deps

import (
	"go.uber.org/dig"

	"package-operator.run/package-operator/cmd/kubectl-package/rootcmd"
)

func Build() (*dig.Container, error) {
	container := dig.New()

	if err := provide(container); err != nil {
		return nil, err
	}

	return container, nil
}

func provide(container *dig.Container) error {
	if err := container.Provide(rootcmd.ProvideRootCmd); err != nil {
		return err
	}
	if err := container.Provide(ProvideIOStreams); err != nil {
		return err
	}
	if err := container.Provide(ProvideArgs); err != nil {
		return err
	}
	if err := container.Provide(ProvideTreeCmd); err != nil {
		return err
	}
	if err := container.Provide(ProvideUpdateCmd); err != nil {
		return err
	}
	if err := container.Provide(ProvideValidateCmd); err != nil {
		return err
	}
	if err := container.Provide(ProvideBuildCmd); err != nil {
		return err
	}
	if err := container.Provide(ProvideVersionCmd); err != nil {
		return err
	}
	if err := container.Provide(ProvideLogFactory); err != nil {
		return err
	}
	if err := container.Provide(ProvideScheme); err != nil {
		return err
	}
	if err := container.Provide(ProvideUpdater); err != nil {
		return err
	}
	if err := container.Provide(ProvideBuilderFactory); err != nil {
		return err
	}
	if err := container.Provide(ProvideValidator); err != nil {
		return err
	}

	return container.Provide(ProvideRendererFactory)
}
