package versioncmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"package-operator.run/package-operator/internal/version"
)

const (
	versionUse         = "version"
	versionShort       = "Output build info of the application"
	versionEmbeddedUse = "Output embedded build information as wel"
)

type Version struct {
	Embedded bool
}

func (v Version) Run(out io.Writer) {
	info := version.Get()

	if info.ApplicationVersion != "" {
		fmt.Fprintln(out, "version", info.ApplicationVersion)
	}

	if v.Embedded {
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

func (v *Version) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{Use: versionUse, Short: versionShort}
	f := cmd.Flags()
	f.BoolVar(&v.Embedded, "embedded", false, versionEmbeddedUse)

	cmd.Run = func(cmd *cobra.Command, args []string) { v.Run(cmd.OutOrStdout()) }

	return cmd
}
