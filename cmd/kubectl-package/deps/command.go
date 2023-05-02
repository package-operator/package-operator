package deps

import (
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/dig"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/package-operator/cmd/kubectl-package/buildcmd"
	"package-operator.run/package-operator/cmd/kubectl-package/rootcmd"
	"package-operator.run/package-operator/cmd/kubectl-package/treecmd"
	"package-operator.run/package-operator/cmd/kubectl-package/updatecmd"
	"package-operator.run/package-operator/cmd/kubectl-package/validatecmd"
	"package-operator.run/package-operator/cmd/kubectl-package/versioncmd"
	internalcmd "package-operator.run/package-operator/internal/cmd"
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

func ProvideBuilderFactory(scheme *runtime.Scheme, f LogFactory) buildcmd.BuilderFactory {
	return &defaultBuilderFactory{
		scheme:     scheme,
		logFactory: f,
	}
}

type defaultBuilderFactory struct {
	scheme     *runtime.Scheme
	logFactory LogFactory
}

func (f *defaultBuilderFactory) Builder() buildcmd.Builder {
	return internalcmd.NewBuild(
		f.scheme,
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
