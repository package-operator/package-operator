package rolloutcmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/cmd/cmdutil"
)

const (
	historyUse   = "history object/name"
	historyShort = "view object rollout history"
	historyLong  = "view previous object rollout revisions and configurations"
	namespaceUse = "If present, the namespace scope for this CLI request."
	revisionUse  = "View a specific revision"
	outputUse    = "Output format. One of: (json, yaml, name)."
)

type History struct {
	Name           string
	Namespace      string
	Object         string
	ObjectFullName string
	Output         string
	Revision       int64
}

func (h *History) Complete(args []string) error {
	switch {
	case len(args) != 1:
		return fmt.Errorf("%w: got %v positional args. Need one argument containing 'object/name'", cmdutil.ErrInvalidArgs, len(args))
	case args[0] == "":
		return fmt.Errorf("%w: package name empty", cmdutil.ErrInvalidArgs)
	}
	split_arg := strings.Split(args[0], "/")
	if len(split_arg) != 2 {
		return fmt.Errorf("%w: cannot parse. Need one argument containing 'object/name'", cmdutil.ErrInvalidArgs)
	}

	h.Object = split_arg[0]
	h.Name = split_arg[1]
	return nil
}

func (h *History) Run(ctx context.Context, out io.Writer) error {
	verboseLog := logr.FromContextOrDiscard(ctx).V(1)
	verboseLog.Info("looking up rollout history for", h.Object, "/", h.Name)

	var clusterRevisions *[]v1alpha1.ClusterObjectSet
	var namespacedRevisions *[]v1alpha1.ObjectSet
	var err error

	var kubeClient client.Client

	kubeClient, err = client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: cmdutil.Scheme,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	var object string

	switch h.Object {
	case "clusterpackage", "cpkg":
		clusterRevisions, err = GetClusterHistory(ctx, kubeClient, h.Name, v1alpha1.ClusterPackage{})
		if err != nil {
			return fmt.Errorf("retrieving objectsets: %w", err)
		}
		object = "clusterpackages.package-operator.run"
	case "clusterobjectdeployment", "cobjdeploy":
		clusterRevisions, err = GetClusterHistory(ctx, kubeClient, h.Name, v1alpha1.ClusterObjectSet{})
		if err != nil {
			return fmt.Errorf("retrieving objectsets: %w", err)
		}
		object = "clusterobjectdeployments.package-operator.run"
	case "package", "pkg":
		namespacedRevisions, err = GetNamespacedHistory(ctx, kubeClient, h.Name, h.Namespace, v1alpha1.Package{})
		if err != nil {
			return fmt.Errorf("retrieving objectsets: %w", err)
		}
		object = "packages.package-operator.run"
	case "objectdeployment", "objdeploy":
		namespacedRevisions, err = GetNamespacedHistory(ctx, kubeClient, h.Name, h.Namespace, v1alpha1.ObjectDeployment{})
		if err != nil {
			return fmt.Errorf("retrieving objectsets: %w", err)
		}
		object = "objectdeployments.package-operator.run"
	default:
		return fmt.Errorf("%w: invalid object. Needs to be one of clusterpackage,clusterobjectdeployment,package,objectdeployment", cmdutil.ErrInvalidArgs)
	}

	if clusterRevisions == nil && namespacedRevisions != nil {
		if h.Revision > 0 {
			revision, err := GetNamespacedRevision(namespacedRevisions, h.Revision)
			if err != nil {
				return fmt.Errorf("retrieving objectsets: %w", err)
			}
			err = PrintRevision(object, h.Name, revision, h.Output)
			if err != nil {
				return fmt.Errorf("printing revision: %w", err)
			}
		} else {
			err = PrintHistory(object, h.Name, namespacedRevisions, h.Output)
			if err != nil {
				return fmt.Errorf("printing history: %w", err)
			}
		}
	} else if clusterRevisions != nil && namespacedRevisions == nil {
		if h.Revision > 0 {
			revision, err := GetClusterRevision(clusterRevisions, h.Revision)
			if err != nil {
				return fmt.Errorf("retrieving objectsets: %w", err)
			}
			err = PrintClusterRevision(object, h.Name, revision, h.Output)
			if err != nil {
				return fmt.Errorf("printing revision: %w", err)
			}
		} else {
			err = PrintClusterHistory(object, h.Name, clusterRevisions, h.Output)
			if err != nil {
				return fmt.Errorf("printing history: %w", err)
			}
		}
	}

	return nil
}

func (h *History) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   historyUse,
		Short: historyShort,
		Long:  historyLong,
	}
	f := cmd.Flags()
	f.StringVarP(&h.Namespace, "namespace", "n", "", namespaceUse)
	f.Int64VarP(&h.Revision, "revision", "r", 0, revisionUse)
	f.StringVarP(&h.Output, "output", "o", "", outputUse)

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
