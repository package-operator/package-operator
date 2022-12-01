package command

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	"package-operator.run/package-operator/cmd/kubectl-package/export"
	"package-operator.run/package-operator/internal/packages/packagebytes"
	"package-operator.run/package-operator/internal/packages/packagestructure"
)

const (
	buildUse       = "build source_path [--tag tag]... [--output output_path] [--push]"
	buildShort     = "build an PKO package image using manifests at the given path"
	buildLong      = "builds and optionally pushes an OCI image in the Package Operator package format from the specified build context directory."
	buildTagUse    = "Tags to assign to the created image. May be specified multiple times. Defaults to none."
	buildPushUse   = "Push the created image tags. Defaults to false"
	buildOutputUse = "Filesystem path to dump the tagged to. Containing directories must exist. Defaults to none."
)

type Build struct {
	SourcePath string
	OutputPath string
	Tags       []name.Reference
	Push       bool
	tags       []string
}

func (b *Build) Complete(args []string) (err error) {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing the source path", ErrInvalidArgs, len(args))
	case (b.OutputPath != "" || b.Push) && len(b.tags) == 0:
		return fmt.Errorf("%w: output or push is requested but no tags are set", ErrInvalidArgs)
	case args[0] == "":
		return fmt.Errorf("%w: source path empty", ErrInvalidArgs)
	}

	b.Tags = make([]name.Reference, len(b.tags))
	for i, stringReference := range b.tags {
		b.Tags[i], err = name.ParseReference(stringReference)
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

	loader := packagebytes.NewLoader()
	saver := packagebytes.NewSaver()

	fileMap, err := loader.FromFS(ctx, os.DirFS(b.SourcePath))
	if err != nil {
		return fmt.Errorf("load source from disk path %s: %w", b.SourcePath, err)
	}

	verboseLog.Info("creating image")

	image, err := saver.ToImage(fileMap)
	if err != nil {
		return fmt.Errorf("image source: %w", err)
	}

	verboseLog.Info("validating package image")

	structureLoaderOpts := []packagestructure.LoaderOption{packagestructure.WithManifestValidators(&packagestructure.ObjectPhaseAnnotationValidator{})}
	structureLoader := packagestructure.NewLoader(validateScheme, structureLoaderOpts...)

	if _, err := structureLoader.LoadFromImage(ctx, image); err != nil {
		return err
	}

	if b.OutputPath != "" {
		verboseLog.Info("writing tagged image to disk", "path", b.OutputPath)

		if err := export.ComressedTarToDisk(b.OutputPath, b.Tags, image); err != nil {
			return err
		}
	}

	if b.Push {
		if err := export.Push(ctx, b.Tags, image, nil); err != nil {
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
	f.StringSliceVarP(&b.tags, "tag", "t", []string{}, buildTagUse)
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
