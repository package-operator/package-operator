package rootcmd

import (
	"flag"
	"io"

	"github.com/spf13/cobra"

	"go.uber.org/dig"

	"package-operator.run/internal/version"
)

type Params struct {
	dig.In

	Streams     IOStreams
	Args        []string
	SubCommands []*cobra.Command `group:"rootSubCommands"`
}

type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

func ProvideRootCmd(params Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kubectl-package",
		Version:      version.Get().ApplicationVersion,
		SilenceUsage: true,
	}
	cmd.SetIn(params.Streams.In)
	cmd.SetOut(params.Streams.Out)
	cmd.SetErr(params.Streams.ErrOut)
	cmd.SetArgs(params.Args)
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	for _, sub := range params.SubCommands {
		cmd.AddCommand(sub)
	}

	return cmd
}
