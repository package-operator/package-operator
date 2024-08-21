package repocmd

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repository",
		Short:   "interact with package repositories",
		Aliases: []string{"repo"},
	}

	cmd.AddCommand(newInitCmd(), newPullCmd(), newAddCmd(), newRemoveCmd(), newPushCmd())

	return cmd
}
