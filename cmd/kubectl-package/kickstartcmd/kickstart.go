package kickstartcmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Kickstarter interface {
	Kickstart(
		ctx context.Context, pkgName string,
		inputs []string, params []string,
		olmBundle string,
	) (msg string, err error)
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

		msg, err := kickstarter.Kickstart(cmd.Context(), args[0], opts.Inputs, opts.Params, opts.OLMBundle)
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
	// Parametrize inputs.
	Params []string
	// OLM Bundle.
	OLMBundle string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	const (
		inputUse       = "Files or urls to load objects from. Supports glob and \"-\" to read from stdin."
		parametrizeUse = "Parametrize flags: e.g. replicas."
		olmBundle      = "OLM Bundle OCI to import. e.g. quay.io/xx/xxx:tag"
	)

	flags.StringSliceVarP(
		&o.Inputs,
		"filename",
		"f",
		nil,
		inputUse,
	)

	flags.StringSliceVarP(
		&o.Params,
		"parametrize",
		"p",
		nil,
		inputUse,
	)

	flags.StringVar(
		&o.OLMBundle,
		"olm-bundle",
		"",
		olmBundle,
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
