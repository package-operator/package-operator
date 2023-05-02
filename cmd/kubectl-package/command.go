package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"package-operator.run/package-operator/cmd/kubectl-package/buildcmd"
	"package-operator.run/package-operator/cmd/kubectl-package/treecmd"
	"package-operator.run/package-operator/cmd/kubectl-package/updatecmd"
	"package-operator.run/package-operator/cmd/kubectl-package/validatecmd"
	"package-operator.run/package-operator/cmd/kubectl-package/versioncmd"
	"package-operator.run/package-operator/internal/version"
)

const (
	// ReturnCodeSuccess is passed to os.Exit() when no error is reported.
	ReturnCodeSuccess = 0
	// ReturnCodeError is passed to os.Exit() if a command report an error.
	ReturnCodeError = 1
)

func Run(ctx context.Context, inReader io.Reader, outWriter, errWriter io.Writer, args []string) int {
	cmd := CobraRoot()
	cmd.SetIn(inReader)
	cmd.SetOut(outWriter)
	cmd.SetErr(errWriter)
	cmd.SetArgs(args)

	if err := cmd.ExecuteContext(ctx); err != nil {
		return ReturnCodeError
	}

	return ReturnCodeSuccess
}

func CobraRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kubectl-package",
		Version:      version.Get().ApplicationVersion,
		SilenceUsage: true,
	}

	// Add additional subcommands here. Bear in mind that the top level context
	// can be fetched by calling the Context method of the command reference that
	// is passed to all RunX methods.
	cmd.AddCommand(
		buildcmd.NewCmd(),
		validatecmd.NewCmd(),
		versioncmd.NewCmd(),
		treecmd.NewCmd(),
		updatecmd.NewCmd(),
	)

	return cmd
}
