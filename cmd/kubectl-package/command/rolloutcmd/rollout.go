package rolloutcmd

import (
	"github.com/spf13/cobra"

	"package-operator.run/package-operator/cmd/kubectl-package/command/rolloutcmd/historycmd"
)

const (
	rolloutUse   = "rollout SUBCOMMAND"
	rolloutShort = "rollout manages a package"
	rolloutLong  = "rollout manages a package using subcommands like 'kubectl rollout history foo/package'"
)

func CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   rolloutUse,
		Short: rolloutShort,
		Long:  rolloutLong,
	}
	cmd.AddCommand(
		historycmd.CobraCommand(),
	)

	return cmd
}
