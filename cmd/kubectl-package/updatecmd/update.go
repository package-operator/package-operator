package updatecmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	internalcmd "package-operator.run/internal/cmd"
)

type Updater interface {
	UpdateLockData(ctx context.Context, srcPath string, opts ...internalcmd.GenerateLockDataOption) (err error)
}

func NewCmd(updater Updater) *cobra.Command {
	const (
		updateUse            = "update source_path"
		updateShort          = "updates image digests of the specified package"
		updateLong           = "updates image digests of the specified package storing them in the manifest.lock file"
		updateSuccessMessage = "Package updated successfully!"
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
		srcPath := args[0]

		if srcPath == "" {
			return fmt.Errorf("%w: target path empty", internalcmd.ErrInvalidArgs)
		}

		err = updater.UpdateLockData(cmd.Context(), srcPath, internalcmd.WithInsecure(opts.Insecure))

		switch {
		case err == nil:
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), updateSuccessMessage); err != nil {
				panic(err)
			}

			return nil
		case errors.Is(err, internalcmd.ErrLockDataUnchanged):
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Package is already up-to-date"); err != nil {
				panic(err)
			}

			return nil
		default:
			return fmt.Errorf("generating lock data: %w", err)
		}
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
