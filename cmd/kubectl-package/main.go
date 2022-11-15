package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/spf13/cobra"
	"package-operator.run/package-operator/internal/version"
)

const (
	// ReturnCodeSuccess is passed to os.Exit() when no error is reported.
	ReturnCodeSuccess = 0
	// ReturnCodeError is passed to os.Exit() if a command report an error.
	ReturnCodeError = 1
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	cmd := &cobra.Command{
		Use:           "kubectl-package [command]",
		SilenceErrors: true, // Do not output errors, we do that.
		SilenceUsage:  true, // Do not output usage on error.
	}

	// Add additional subcommands here. Bear in mind that the top level context
	// can be fetched by calling the Context method of the command reference that
	// is passed to all RunX methods.
	cmd.AddCommand(versionCommand())

	err := cmd.ExecuteContext(ctx)
	cancel()

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(ReturnCodeError)
	}

	os.Exit(ReturnCodeSuccess)
}

func versionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Output build info of the application",
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		versionInfo := version.Get()
		buildInfo := versionInfo.BuildInfo
		// Slap our version info into the BuildInfo settings slice so it gets formatted consistently in the next step.
		buildInfo.Settings = append(buildInfo.Settings, debug.BuildSetting{Key: "vcs.version", Value: versionInfo.Version})

		// Let BuildInfo format itself.
		fmt.Print(buildInfo) //nolint: forbidigo
	}

	return cmd
}
