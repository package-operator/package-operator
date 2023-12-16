package repocmd

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"

	internalcmd "package-operator.run/internal/cmd"
	"package-operator.run/internal/packages"
)

func newPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull file tag",
		Short: "pull a repository from tag and write it to file",
		Args:  cobra.ExactArgs(2),
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		filePath := args[0]
		tag := args[1]

		switch {
		case tag == "":
			return fmt.Errorf("%w: tag must be not empty", internalcmd.ErrInvalidArgs)
		case filePath == "":
			return fmt.Errorf("%w: file must be not empty", internalcmd.ErrInvalidArgs)
		}

		image, err := crane.Pull(tag)
		if err != nil {
			return fmt.Errorf("pull repository image: %w", err)
		}

		idx, err := packages.LoadRepositoryFromOCI(ctx, image)
		if err != nil {
			return err
		}

		if err := packages.SaveRepositoryToFile(ctx, filePath, idx); err != nil {
			return fmt.Errorf("write to file: %w", err)
		}

		return nil
	}

	return cmd
}
