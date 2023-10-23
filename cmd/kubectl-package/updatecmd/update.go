package updatecmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/internal/cmd"
	"package-operator.run/internal/packages"
)

type Updater interface {
	GenerateLockData(
		ctx context.Context, srcPath string, opts ...internalcmd.GenerateLockDataOption,
	) (data []byte, err error)
}

func NewCmd(updater Updater) *cobra.Command {
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

	var opts options

	opts.AddFlags(cmd.Flags())

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if args[0] == "" {
			return fmt.Errorf("%w: target path empty", internalcmd.ErrInvalidArgs)
		}

		srcPath := args[0]

		data, err := updater.GenerateLockData(cmd.Context(), srcPath, internalcmd.WithInsecure(opts.Insecure))
		if errors.Is(err, internalcmd.ErrLockDataUnchanged) {
			fmt.Fprintln(cmd.OutOrStdout(), "Package is already up-to-date")

			return nil
		} else if err != nil {
			return fmt.Errorf("generating lock data: %w", err)
		}

		lockFilePath := filepath.Join(srcPath, packages.PackageManifestLockFilename+".yaml")
		if err := os.WriteFile(lockFilePath, data, 0o644); err != nil {
			return fmt.Errorf("writing lock file: %w", err)
		}

		return nil
	}

	return cmd
}

type options struct {
	Insecure bool
	Pull     bool
}

func (o *options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(
		&o.Insecure,
		"insecure",
		o.Insecure,
		"Allows pulling images without TLS or using TLS with unverified certificates.",
	)
}
