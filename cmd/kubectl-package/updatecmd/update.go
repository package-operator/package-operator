package updatecmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	internalcmd "package-operator.run/package-operator/internal/cmd"
	"package-operator.run/package-operator/internal/packages"
)

func NewCmd() *cobra.Command {
	const (
		updateUse   = "update source_path"
		updateShort = "updates image digests of the specified package"
		updateLong  = "updates image digests of the specified package storing them in the manifest.lock file"
	)

	cmd := &cobra.Command{
		Use:   updateUse,
		Short: updateShort,
		Long:  updateLong,
		Args:  cobra.ExactArgs(1),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if args[0] == "" {
			return fmt.Errorf("%w: target path empty", internalcmd.ErrInvalidArgs)
		}

		srcPath := args[0]
		update := internalcmd.NewUpdate()

		data, err := update.GenerateLockData(cmd.Context(), srcPath)
		if err != nil {
			return err
		}

		lockFilePath := filepath.Join(srcPath, packages.PackageManifestLockFile)
		if err := os.WriteFile(lockFilePath, data, 0o644); err != nil {
			return fmt.Errorf("writing lock file: %w", err)
		}

		return nil
	}

	return cmd
}
