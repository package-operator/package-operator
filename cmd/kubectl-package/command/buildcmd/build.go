package buildcmd

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	"package-operator.run/package-operator/cmd/kubectl-package/command/cmdutil"
	"package-operator.run/package-operator/internal/packages/packageexport"
	"package-operator.run/package-operator/internal/packages/packageimport"
	"package-operator.run/package-operator/internal/packages/packageloader"
)

const (
	buildUse       = "build source_path [--tag tag]... [--output output_path] [--push]"
	buildShort     = "build an PKO package image using manifests at the given path"
	buildLong      = "builds and optionally pushes an OCI image in the Package Operator package format from the specified build context directory."
	buildTagUse    = "Tags to assign to the created image. May be specified multiple times. Defaults to none."
	buildPushUse   = "Push the created image tags. Defaults to false"
	buildOutputUse = "Filesystem path to dump the tagged image to. Will be packed as a tar. Containing directories must exist. Defaults to none."
)

type Build struct {
	SourcePath string
	OutputPath string
	Tags       []string
	Push       bool
}

func (b *Build) Complete(args []string) (err error) {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing the source path", cmdutil.ErrInvalidArgs, len(args))
	case (b.OutputPath != "" || b.Push) && len(b.Tags) == 0:
		return fmt.Errorf("%w: output or push is requested but no tags are set", cmdutil.ErrInvalidArgs)
	case args[0] == "":
		return fmt.Errorf("%w: source path empty", cmdutil.ErrInvalidArgs)
	}

	for _, stringReference := range b.Tags {
		_, err = name.ParseReference(stringReference)
		if err != nil {
			return fmt.Errorf("invalid tag specified as parameter %s: %w", stringReference, err)
		}
	}
	b.SourcePath = args[0]

	return nil
}

func (b Build) Run(ctx context.Context) error {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	verboseLog.Info("loading source from disk", "path", b.SourcePath)

	files, err := packageimport.Folder(ctx, b.SourcePath)
	if err != nil {
		return fmt.Errorf("load source from disk path %s: %w", b.SourcePath, err)
	}

	verboseLog.Info("creating image")

	loader := packageloader.New(cmdutil.ValidateScheme, packageloader.WithDefaults)

	if _, err := loader.FromFiles(ctx, files); err != nil {
		return err
	}

	if b.OutputPath != "" {
		verboseLog.Info("writing tagged image to disk", "path", b.OutputPath)

		if err := packageexport.File(b.OutputPath, b.Tags, files); err != nil {
			return err
		}
	}

	if b.Push {
		if err := packageexport.PushedImage(ctx, b.Tags, files); err != nil {
			return err
		}
	}

	return nil
}

func (b *Build) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   buildUse,
		Short: buildShort,
		Long:  buildLong,
	}
	f := cmd.Flags()
	f.StringSliceVarP(&b.Tags, "tag", "t", []string{}, buildTagUse)
	f.BoolVar(&b.Push, "push", false, buildPushUse)
	f.StringVarP(&b.OutputPath, "output", "o", "", buildOutputUse)

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if err := b.Complete(args); err != nil {
			return err
		}
		logOut := cmd.ErrOrStderr()
		log := funcr.New(func(p, a string) { fmt.Fprintln(logOut, p, a) }, funcr.Options{})
		return b.Run(logr.NewContext(cmd.Context(), log))
	}

	return cmd
}
