package repocmd

import (
	"fmt"

	"github.com/spf13/cobra"

	internalcmd "package-operator.run/internal/cmd"
	"package-operator.run/internal/packages"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "initialize file name",
		Short:   "init a new repository in file with name",
		Args:    cobra.ExactArgs(2),
		Aliases: []string{"init"},
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		filePath := args[0]
		repoName := args[1]

		switch {
		case repoName == "":
			return fmt.Errorf("%w: name must be not empty", internalcmd.ErrInvalidArgs)
		case filePath == "":
			return fmt.Errorf("%w: file must be not empty", internalcmd.ErrInvalidArgs)
		}

		repo := packages.NewRepositoryIndex(metav1.ObjectMeta{Name: repoName})

		if err := packages.SaveRepositoryToFile(ctx, filePath, repo); err != nil {
			return fmt.Errorf("write to file: %w", err)
		}

		return nil
	}

	return cmd
}
