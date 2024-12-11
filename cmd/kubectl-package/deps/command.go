package deps

import (
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/dig"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/cmd/kubectl-package/buildcmd"
	clustertreecmd "package-operator.run/cmd/kubectl-package/clustertreecmd"
	"package-operator.run/cmd/kubectl-package/kickstartcmd"
	"package-operator.run/cmd/kubectl-package/rolloutcmd"
	"package-operator.run/cmd/kubectl-package/rootcmd"
	"package-operator.run/cmd/kubectl-package/treecmd"
	"package-operator.run/cmd/kubectl-package/updatecmd"
	"package-operator.run/cmd/kubectl-package/validatecmd"
	"package-operator.run/cmd/kubectl-package/versioncmd"
	internalcmd "package-operator.run/internal/cmd"
)

func ProvideIOStreams() rootcmd.IOStreams {
	return rootcmd.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}

func ProvideArgs() []string {
	return os.Args[1:]
}

type RootSubCommandResult struct {
	dig.Out

	SubCommand *cobra.Command `group:"rootSubCommands"`
}

func ProvideTreeCmd(rendererFactory treecmd.RendererFactory) RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: treecmd.NewCmd(
			rendererFactory,
		),
	}
}

func ProvideClusterTreeCmd(clientFactory internalcmd.ClientFactory) RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: clustertreecmd.NewClusterTreeCmd(clientFactory),
	}
}

func ProvideRendererFactory(scheme *runtime.Scheme, f LogFactory) treecmd.RendererFactory {
	return &defaultRendererFactory{
		logFactory: f,
		scheme:     scheme,
	}
}

type defaultRendererFactory struct {
	logFactory LogFactory
	scheme     *runtime.Scheme
}

func (f *defaultRendererFactory) Renderer() treecmd.Renderer {
	return internalcmd.NewTree(
		f.scheme,
		internalcmd.WithLog{
			Log: f.logFactory.Logger(),
		},
	)
}

func ProvideUpdateCmd(updater updatecmd.Updater) RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: updatecmd.NewCmd(
			updater,
		),
	}
}

func ProvideUpdater(scheme *runtime.Scheme) updatecmd.Updater {
	return internalcmd.NewUpdate(
		internalcmd.WithPackageLoader{Loader: internalcmd.NewDefaultPackageLoader(
			scheme,
		)},
	)
}

func ProvideValidateCmd(validator validatecmd.Validator) RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: validatecmd.NewCmd(
			validator,
		),
	}
}

func ProvideValidator(scheme *runtime.Scheme) validatecmd.Validator {
	return internalcmd.NewValidate(scheme)
}

func ProvideBuildCmd(builderFactory buildcmd.BuilderFactory) RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: buildcmd.NewCmd(
			builderFactory,
		),
	}
}

func ProvideBuilderFactory(f LogFactory) buildcmd.BuilderFactory {
	return &defaultBuilderFactory{
		logFactory: f,
	}
}

type defaultBuilderFactory struct {
	logFactory LogFactory
}

func (f *defaultBuilderFactory) Builder() buildcmd.Builder {
	return internalcmd.NewBuild(
		internalcmd.WithLog{
			Log: f.logFactory.Logger(),
		},
	)
}

func ProvideVersionCmd() RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: versioncmd.NewCmd(),
	}
}

func ProvideRolloutCmd(params rolloutcmd.Params) RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: rolloutcmd.NewRolloutCmd(params),
	}
}

func ProvideKickstartCmd(ks kickstartcmd.Kickstarter) RootSubCommandResult {
	return RootSubCommandResult{
		SubCommand: kickstartcmd.NewCmd(ks),
	}
}

type RolloutSubCommandResult struct {
	dig.Out

	SubCommand *cobra.Command `group:"rolloutSubCommands"`
}

func ProvideRolloutHistoryCmd(clientFactory internalcmd.ClientFactory) RolloutSubCommandResult {
	return RolloutSubCommandResult{
		SubCommand: rolloutcmd.NewHistoryCmd(clientFactory),
	}
}

func ProvideRolloutRollbackCmd(clientFactory internalcmd.ClientFactory) RolloutSubCommandResult {
	return RolloutSubCommandResult{
		SubCommand: rolloutcmd.NewRollbackCmd(clientFactory),
	}
}

func ProvideClientFactory(kcliFactory internalcmd.KubeClientFactory) internalcmd.ClientFactory {
	return internalcmd.NewDefaultClientFactory(kcliFactory)
}

func ProvideKickstarter() kickstartcmd.Kickstarter {
	return internalcmd.NewKickstarter(os.Stdin)
}
