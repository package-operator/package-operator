package repocmd

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"

	internalcmd "package-operator.run/internal/cmd"
	"package-operator.run/internal/packages"
)

var ErrInvalidImage = errors.New("image does not contain repository")

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

		reader := mutate.Extract(image)

		defer func() {
			if cErr := reader.Close(); err == nil && cErr != nil {
				err = cErr
			}
		}()
		tarReader := tar.NewReader(reader)

		for {
			hdr, err := tarReader.Next()
			switch {
			case err == nil:
			case errors.Is(err, io.EOF):
				return ErrInvalidImage
			default:
				return fmt.Errorf("read from image tar: %w", err)
			}

			if hdr.Name != filePathInRepo {
				continue
			}

			idx, err := packages.LoadRepository(ctx, tarReader)
			if err != nil {
				return err
			}

			if err := packages.SaveRepositoryToFile(ctx, filePath, idx); err != nil {
				return fmt.Errorf("write to file: %w", err)
			}
		}
	}

	return cmd
}
