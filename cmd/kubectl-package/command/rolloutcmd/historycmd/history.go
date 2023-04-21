package historycmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/package-operator/cmd/cmdutil"
)

const (
	historyUse             = "history package_name"
	historyShort           = "view package rollout history"
	historyLong            = "view previous package rollout revisions and configurations"
	historyClusterScopeUse = "render in cluster scope"
)

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

	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: cmdutil.Scheme,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}
	var packageList corev1alpha1.ObjectSetList
	err = c.List(ctx, &packageList, client.InNamespace("default"))
	if err != nil {
		return fmt.Errorf("getting packages: %w", err)
	}
	Marshall(packageList)

	return nil
}

func Marshall(obj corev1alpha1.ObjectSetList) {
	//Marshal
	empJSON, err := json.Marshal(obj)
	if err != nil {
		log.Fatalf(err.Error())
	}
	fmt.Printf("%s\n", string(empJSON))
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
