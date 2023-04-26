package treecmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/package-operator/internal/cmd"
)

func NewCmd() *cobra.Command {
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
		tree := internalcmd.NewTree(
			internalcmd.WithLog{
				Log: internalcmd.LogFromCmd(cmd).V(1),
			},
		)

		out, err := tree.RenderPackage(
			cmd.Context(), args[0],
			internalcmd.WithClusterScope(opts.ClusterScope),
			internalcmd.WithConfigPath(opts.ConfigPath),
			internalcmd.WithConfigTestcase(opts.ConfigTestcase),
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
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	const (
		clusterScopeUse   = "render package in cluster scope"
		configTestcaseUse = "name of the testcase which config is for templating"
		configPathUse     = "file containing config which is used for templating."
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
}
