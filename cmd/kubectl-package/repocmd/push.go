package repocmd

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"

	internalcmd "package-operator.run/internal/cmd"
	"package-operator.run/internal/packages"
)

func newPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push file tag",
		Short: "push a repository from file to tag",
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

		idx, err := packages.LoadRepositoryFromFile(ctx, filePath)
		if err != nil {
			return fmt.Errorf("read from file: %w", err)
		}

		image, err := packages.SaveRepositoryToOCI(ctx, idx)
		if err != nil {
			return err
		}

		if err := crane.Push(image, tag); err != nil {
			return fmt.Errorf("pull repository image: %w", err)
		}

		return nil
	}

	return cmd
}
