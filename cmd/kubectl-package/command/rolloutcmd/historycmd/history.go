package historycmd

import (
	"context"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"
)

const (
	historyUse   = "history PACKAGE"
	historyShort = "view package rollout history"
	historyLong  = "view previous package rollout revisions and configurations"
)

func Run(ctx context.Context, out io.Writer) error {
	fmt.Println("Hello there!")

	return nil
}

func CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   historyUse,
		Short: historyShort,
		Long:  historyLong,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		logOut := cmd.ErrOrStderr()
		log := funcr.New(func(p, a string) { fmt.Fprintln(logOut, p, a) }, funcr.Options{})
		return Run(logr.NewContext(cmd.Context(), log), cmd.OutOrStdout())
	}

	return cmd
}
