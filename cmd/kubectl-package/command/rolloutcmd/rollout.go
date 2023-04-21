package rolloutcmd

import (
	"github.com/spf13/cobra"
)

const (
	rolloutUse   = "rollout SUBCOMMAND"
	rolloutShort = "rollout manages a package"
	rolloutLong  = "rollout manages a package using subcommands like 'kubectl package rollout history foo/package'"
)

func CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   rolloutUse,
		Short: rolloutShort,
		Long:  rolloutLong,
	}
	cmd.AddCommand(
		(&History{}).CobraCommand(),
	)

	return cmd
}
