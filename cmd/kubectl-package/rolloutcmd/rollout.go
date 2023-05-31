package rolloutcmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

type Params struct {
	dig.In

	SubCommands []*cobra.Command `group:"rolloutSubCommands"`
}

func NewRolloutCmd(params Params) *cobra.Command {
	const (
		cmdUse   = "rollout"
		cmdShort = "view package rollout status or history"
		cmdLong  = "view package rollout status or history including detailed revision information"
	)

	cmd := &cobra.Command{
		Use:   cmdUse,
		Short: cmdShort,
		Long:  cmdLong,
	}

	for _, sub := range params.SubCommands {
		cmd.AddCommand(sub)
	}

	return cmd
}
