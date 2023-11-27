package repocmd

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add file package version...",
		Short: "adds package with versions to the repository at file",
		Args:  cobra.MinimumNArgs(3),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		filePath := args[0]
		packageTagStr := args[1]

		packageTag, err := name.ParseReference(packageTagStr, name.StrictValidation)
		if err != nil {
			return fmt.Errorf("package reference: %w", err)
		}

		versionsStrs := args[2:]
		for _, v := range versionsStrs {
			if _, err := semver.StrictNewVersion(v); err != nil {
				return fmt.Errorf("version: %w", err)
			}
		}

		idx, err := packages.LoadRepositoryFromFile(ctx, filePath)
		if err != nil {
			return err
		}

		pkgImg, err := crane.Pull(packageTagStr)
		if err != nil {
			return fmt.Errorf("pull package image: %w", err)
		}

		rawPkg, err := packages.FromOCI(ctx, pkgImg)
		if err != nil {
			return fmt.Errorf("raw package from package image: %w", err)
		}

		pkg, err := packages.DefaultStructuralLoader.Load(ctx, rawPkg)
		if err != nil {
			return fmt.Errorf("package from raw package: %w", err)
		}

		digest, err := pkgImg.Digest()
		if err != nil {
			return fmt.Errorf("pulled package: %w", err)
		}

		entry := &manifests.RepositoryEntry{
			Data: manifests.RepositoryEntryData{
				Image:       packageTag.Context().Name(),
				Digest:      digest.Hex,
				Versions:    versionsStrs,
				Constraints: pkg.Manifest.Spec.Constraints,
				Name:        pkg.Manifest.Name,
			},
		}

		if err := idx.Add(ctx, entry); err != nil {
			return fmt.Errorf("add new entry: %w", err)
		}

		if err := packages.SaveRepositoryToFile(ctx, filePath, idx); err != nil {
			return fmt.Errorf("write to file: %w", err)
		}

		return nil
	}

	return cmd
}
