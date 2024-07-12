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
		cmdLong  = "Starts a new package with the given name containing objects referenced via -f in a new folder <pkg_name>."
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
		if err := opts.Validate(); err != nil {
			return err
		}

		msg, err := kickstarter.Kickstart(cmd.Context(), args[0], opts.Inputs)
		if err != nil {
			return fmt.Errorf("kickstarting package: %w", err)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), msg)
		return err
	}

	return cmd
}

type FlagNeedArgumentError struct {
	flag string
}

func (e *FlagNeedArgumentError) Error() string {
	return fmt.Sprintf("flag needs an argument: '%s' in -%s", e.flag, e.flag)
}

type options struct {
	// Inputs (files/http/etc.)
	Inputs []string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	const (
		inputUse = "Files or urls to load objects from. Supports glob and \"-\" to read from stdin."
	)

	flags.StringSliceVarP(
		&o.Inputs,
		"filename",
		"f",
		nil,
		inputUse,
	)
}

func (o *options) Validate() error {
	for _, i := range o.Inputs {
		if len(i) == 0 {
			return &FlagNeedArgumentError{}
		}
	}
	return nil
}
