package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"package-operator.run/cmd/kubectl-package/deps"
	"package-operator.run/cmd/kubectl-package/loadcmd"
)

const (
	// ReturnCodeSuccess is passed to os.Exit() when no error is reported.
	ReturnCodeSuccess = 0
	// ReturnCodeError is passed to os.Exit() if a command report an error.
	ReturnCodeError = 1
)

func main() {
	rc := ReturnCodeSuccess
	if err := run(); err != nil {
		rc = ReturnCodeError
	}

	os.Exit(rc)
}

func run() error {
	deps, err := deps.Build()
	if err != nil {
		return fmt.Errorf("building deps: %w", err)
	}

	return deps.Invoke(executeRoot)
}

func executeRoot(rootCmd *cobra.Command) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rootCmd.AddCommand(loadcmd.NewLoadCmd())
	return rootCmd.ExecuteContext(ctx)
}
