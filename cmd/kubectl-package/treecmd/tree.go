package treecmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/internal/cmd"
)

type RendererFactory interface {
	Renderer() Renderer
}

type Renderer interface {
	RenderPackage(ctx context.Context, srcPath string, opts ...internalcmd.RenderPackageOption) (string, error)
}

func NewCmd(rendererFactory RendererFactory) *cobra.Command {
	const (
		cmdUse   = "tree source_path"
		cmdShort = "outputs a logical tree view of the package contents"
		cmdLong  = "outputs a logical tree view of the package by printing root->phases->objects"
	)

	var opts options

	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   cmdUse,
		Short: cmdShort,
		Long:  cmdLong,
	}
	opts.AddFlags(cmd.Flags())

	cmd.MarkFlagsMutuallyExclusive("config-path", "config-testcase")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		out, err := rendererFactory.Renderer().RenderPackage(
			cmd.Context(), args[0],
			internalcmd.WithClusterScope(opts.ClusterScope),
			internalcmd.WithConfigPath(opts.ConfigPath),
			internalcmd.WithConfigTestcase(opts.ConfigTestcase),
			internalcmd.WithComponent(opts.Component),
		)
		if err != nil {
			return fmt.Errorf("rendering package: %w", err)
		}

		_, err = fmt.Fprint(cmd.OutOrStdout(), out)

		return err
	}

	return cmd
}

type options struct {
	ClusterScope   bool
	ConfigPath     string
	ConfigTestcase string
	Component      string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	const (
		clusterScopeUse   = "render package in cluster scope"
		configTestcaseUse = "name of the testcase which config is for templating"
		configPathUse     = "file containing config which is used for templating."
		componentUse      = "select which component to render"
	)

	flags.BoolVar(
		&o.ClusterScope,
		"cluster",
		o.ClusterScope,
		clusterScopeUse,
	)
	flags.StringVar(
		&o.ConfigPath,
		"config-path",
		o.ConfigPath,
		configPathUse,
	)
	flags.StringVar(
		&o.ConfigTestcase,
		"config-testcase",
		o.ConfigTestcase,
		configTestcaseUse,
	)
	flags.StringVar(
		&o.Component,
		"component",
		o.Component,
		configTestcaseUse,
	)
}
