package versioncmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"package-operator.run/internal/version"
)

func NewCmd() *cobra.Command {
	const (
		versionUse   = "version"
		versionShort = "Output build info of the application"
	)

	cmd := &cobra.Command{
		Use:   versionUse,
		Short: versionShort,
	}

	var opts options

	opts.AddFlags(cmd.Flags())

	cmd.Run = func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()

		info := version.Get()

		if info.ApplicationVersion != "" {
			fmt.Fprintln(out, "version", info.ApplicationVersion)
		}

		if opts.Embedded {
			fmt.Fprintln(out, "go", info.GoVersion)
			fmt.Fprintln(out, "path", info.Path)
			fmt.Fprintln(out, "mod", info.Main)
			for _, dep := range info.Deps {
				fmt.Fprintln(out, "dep", dep.Path, dep.Version)
			}

			for _, setting := range info.Settings {
				fmt.Fprintln(out, "build", setting.Key, setting.Value)
			}
		}
	}

	return cmd
}

type options struct {
	Embedded bool
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(
		&o.Embedded,
		"embedded",
		o.Embedded,
		"Output embedded build information as well",
	)
}
