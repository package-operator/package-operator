package historycmd

import (
	"context"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	pkoapis "package-operator.run/apis"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/cmd/cmdutil"
)

const (
	historyUse             = "history package_name"
	historyShort           = "view package rollout history"
	historyLong            = "view previous package rollout revisions and configurations"
	historyClusterScopeUse = "render in cluster scope"
)

var historyScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(historyScheme); err != nil {
		panic(err)
	}
	if err := manifestsv1alpha1.AddToScheme(historyScheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(historyScheme); err != nil {
		panic(err)
	}
	if err := apiextensions.AddToScheme(historyScheme); err != nil {
		panic(err)
	}
}

type History struct {
	PackageName  string
	ClusterScope bool
}

func (h *History) Complete(args []string) error {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing the package name", cmdutil.ErrInvalidArgs, len(args))
	case args[0] == "":
		return fmt.Errorf("%w: package name empty", cmdutil.ErrInvalidArgs)
	}

	h.PackageName = args[0]
	return nil
}

func (h *History) Run(ctx context.Context, out io.Writer) error {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	verboseLog.Info("loading source from disk", "path", h.PackageName)
	fmt.Printf("History: %+v\n", h)

	return nil
}

func (h *History) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   historyUse,
		Short: historyShort,
		Long:  historyLong,
	}
	f := cmd.Flags()
	f.BoolVar(&h.ClusterScope, "cluster", false, historyClusterScopeUse)

	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		if err := h.Complete(args); err != nil {
			return err
		}
		logOut := cmd.ErrOrStderr()
		log := funcr.New(func(p, a string) { fmt.Fprintln(logOut, p, a) }, funcr.Options{})
		return h.Run(logr.NewContext(cmd.Context(), log), cmd.OutOrStdout())
	}

	return cmd
}
