package kickstartcmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Kickstarter interface {
	Kickstart(ctx context.Context, pkgName string, inputs []string) (msg string, err error)
}

func NewCmd(kickstarter Kickstarter) *cobra.Command {
	const (
		cmdUse   = "kickstart pkg_name (experimental)"
		cmdShort = "Starts a new package with the given name."
		cmdLong  = "Starts a new package, containing objects referenced via -f, " +
			"with the given name in a new folder <pkg_name>."
	)

	var opts options

	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   cmdUse,
		Short: cmdShort,
		Long:  cmdLong,
	}
	opts.AddFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		msg, err := kickstarter.Kickstart(cmd.Context(), args[0], opts.Inputs)
		if err != nil {
			return fmt.Errorf("kickstarting package: %w", err)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), msg)
		return err
	}

	return cmd
}

type options struct {
	// Inputs (files/http/etc.)
	Inputs []string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	const (
		inputUse = "Files or urls to load objects from. " +
			`Supports glob and "-" to read from stdin. Can be supplied multiple times.`
	)

	flags.StringSliceVarP(
		&o.Inputs,
		"filename",
		"f",
		nil,
		inputUse,
	)
}
