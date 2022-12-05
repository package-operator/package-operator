package cmdutil

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"
)

func NewCobraContext(cmd *cobra.Command) context.Context {
	logOut := cmd.ErrOrStderr()
	log := funcr.New(func(p, a string) { fmt.Fprintln(logOut, p, a) }, funcr.Options{})
	return logr.NewContext(cmd.Context(), log)
}
