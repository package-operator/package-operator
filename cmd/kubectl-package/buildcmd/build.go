package buildcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/package-operator/internal/cmd"
)

type BuilderFactory interface {
	Builder() Builder
}

type Builder interface {
	BuildFromSource(ctx context.Context, srcPath string, opts ...internalcmd.BuildFromSourceOption) error
}

func NewCmd(builderFactory BuilderFactory) *cobra.Command {
	const (
		buildUse   = "build source_path [--tag tag]... [--output output_path] [--push]"
		buildShort = "build an PKO package image using manifests at the given path"
		buildLong  = "builds and optionally pushes an OCI image in the Package Operator package format from the specified build context directory."
	)

	cmd := &cobra.Command{
		Use:   buildUse,
		Short: buildShort,
		Long:  buildLong,
		Args:  cobra.ExactArgs(1),
	}

	var opts options

	opts.AddFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		src := args[0]
		if src == "" {
			return fmt.Errorf("%w: source path empty", internalcmd.ErrInvalidArgs)
		}
		if (opts.OutputPath != "" || opts.Push) && len(opts.Tags) == 0 {
			return fmt.Errorf("%w: output or push is requested but no tags are set", internalcmd.ErrInvalidArgs)
		}
		for _, ref := range opts.Tags {
			if _, err = name.ParseReference(ref); err != nil {
				return fmt.Errorf("invalid tag specified as parameter %s: %w", ref, err)
			}
		}

		if err := builderFactory.Builder().BuildFromSource(
			cmd.Context(), src,
			internalcmd.WithOutputPath(opts.OutputPath),
			internalcmd.WithPush(opts.Push),
			internalcmd.WithTags(opts.Tags),
		); err != nil {
			return fmt.Errorf("building from source: %w", err)
		}

		return nil
	}

	return cmd
}

type options struct {
	OutputPath string
	Push       bool
	Tags       []string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.StringSliceVarP(
		&o.Tags,
		"tag",
		"t",
		o.Tags,
		"Tags to assign to the created image. May be specified multiple times. Defaults to none.",
	)
	flags.BoolVar(
		&o.Push,
		"push",
		o.Push,
		"Push the created image tags. Defaults to false",
	)
	flags.StringVarP(
		&o.OutputPath,
		"output",
		"o",
		"",
		strings.Join([]string{
			"Filesystem path to dump the tagged image to.",
			"Will be packed as a tar.",
			"Containing directories must exist.",
			"Defaults to none.",
		}, " "),
	)
}
