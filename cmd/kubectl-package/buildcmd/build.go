package buildcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/internal/cmd"
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
		buildLong  = "builds and optionally pushes an OCI image in the Package Operator" +
			" package format from the specified build context directory."
		buildSuccessMessage = "Package built successfully!"
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
		for label, value := range opts.Labels {
			if value == "" {
				return fmt.Errorf("parsing label %q, value is empty", label)
			}
		}
		factoryOpts := []internalcmd.BuildFromSourceOption{
			internalcmd.WithInsecure(opts.Insecure),
			internalcmd.WithOutputPath(opts.OutputPath),
			internalcmd.WithPush(opts.Push),
			internalcmd.WithTags(opts.Tags),
			internalcmd.WithLabels(opts.Labels),
		}

		switch opts.OutputFormat {
		case "":
		case internalcmd.OutputFormatHuman, internalcmd.OutputFormatDigest:
			factoryOpts = append(factoryOpts, internalcmd.WithOutputFormat(opts.OutputFormat))
		default:
			return fmt.Errorf("unknown output format: %s", opts.OutputFormat)
		}

		if err := builderFactory.Builder().BuildFromSource(cmd.Context(), src, factoryOpts...); err != nil {
			return fmt.Errorf("building from source: %w", err)
		}

		switch opts.OutputFormat {
		case "", internalcmd.OutputFormatHuman:
			if _, err := fmt.Fprint(cmd.OutOrStdout(), buildSuccessMessage); err != nil {
				panic(err)
			}
		case internalcmd.OutputFormatDigest:
		default:
			panic(opts.OutputFormat)
		}

		return nil
	}

	return cmd
}

type options struct {
	Insecure     bool
	OutputPath   string
	OutputFormat string
	Push         bool
	Tags         []string
	Labels       map[string]string
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(
		&o.Insecure,
		"insecure",
		o.Insecure,
		"Allows pushing images without TLS or using TLS with unverified certificates.",
	)
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
	flags.StringVar(&o.OutputFormat, "output-format", internalcmd.OutputFormatHuman,
		strings.Join([]string{
			"Either `human` for regular stdout output or `digest` for only printing ",
			"the image digest of the pushed package image",
		}, " "),
	)
	flags.StringToStringVarP(
		&o.Labels,
		"label",
		"l", o.Labels,
		"Labels to add the OCI. May be specified multiple times. Defaults to none.",
	)
}
