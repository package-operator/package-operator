package repocmd

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages"
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove file package",
		Short: "remove package from the repository at file",
		Args:  cobra.ExactArgs(2),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		filePath := args[0]

		packageTagStr := args[1]
		_, err := name.ParseReference(packageTagStr, name.StrictValidation)
		if err != nil {
			return fmt.Errorf("given package reference: %w", err)
		}

		idx, err := packages.LoadRepositoryFromFile(ctx, filePath)
		if err != nil {
			return fmt.Errorf("read from file: %w", err)
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
				Name:   pkg.Manifest.Name,
				Digest: digest.Hex,
			},
		}

		if err := idx.Remove(ctx, entry); err != nil {
			return fmt.Errorf("add new entry: %w", err)
		}

		if err := packages.SaveRepositoryToFile(ctx, filePath, idx); err != nil {
			return fmt.Errorf("write to file: %w", err)
		}

		return nil
	}

	return cmd
}
