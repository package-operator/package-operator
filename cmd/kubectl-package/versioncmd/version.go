package versioncmd

import (
	"github.com/spf13/cobra"

	"package-operator.run/internal/version"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Output build info of the application",
		Args:  cobra.ExactArgs(0),
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error { return version.Get().Write(cmd.OutOrStdout()) }

	return cmd
}
