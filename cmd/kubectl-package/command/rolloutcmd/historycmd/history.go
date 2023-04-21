package historycmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/package-operator/cmd/cmdutil"
)

const (
	historyUse   = "history object/name"
	historyShort = "view object rollout history"
	historyLong  = "view previous object rollout revisions and configurations"
	namespaceUse = "If present, the namespace scope for this CLI request."
)

type History struct {
	client    client.Client
	Object    string
	Name      string
	Namespace string
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

	var err error
	h.client, err = client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: cmdutil.Scheme,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	switch h.Object {
	case "clusterpackage", "cpkg":
		pkg, err := h.GetClusterPackageByName(ctx, h.Name)
		if err != nil {
			return fmt.Errorf("retrieving packages: %w", err)
		}
		if pkg == nil {
			return fmt.Errorf("packages.package-operator.run \"%s\" not found", h.Name)
		}
		objdeploy, err := h.GetClusterObjectDeploymentByOwner(ctx, pkg.UID)
		if err != nil {
			return fmt.Errorf("retrieving objectdeployments: %w", err)
		}
		if objdeploy == nil {
			return fmt.Errorf("objectdeployment not found with: OwnerReferences.UID=%s", pkg.UID)
		}
		objset, err := h.GetClusterObjectSetByOwner(ctx, objdeploy.UID)
		if err != nil {
			return fmt.Errorf("retrieving objectsets: %w", err)
		}
		if objset == nil {
			return fmt.Errorf("objectsets not found with: OwnerReferences.UID=%s", objdeploy.UID)
		}
		fmt.Println(objset)
	case "clusterobjectdeployment", "cobjdeploy":
		objdeploy, err := h.GetClusterObjectDeploymentByName(ctx, h.Name)
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}
		fmt.Println(objdeploy)
		fmt.Println("")
		objset, err := h.GetClusterObjectSetByOwner(ctx, objdeploy.UID)
		fmt.Println(objset)
	case "package", "pkg":
		pkg, err := h.GetPackageByName(ctx, h.Name)
		if err != nil {
			return fmt.Errorf("retrieving packages: %w", err)
		}
		if pkg == nil {
			return fmt.Errorf("packages.package-operator.run \"%s\" not found", h.Name)
		}
		objdeploy, err := h.GetObjectDeploymentByOwner(ctx, pkg.UID)
		if err != nil {
			return fmt.Errorf("retrieving objectdeployments: %w", err)
		}
		if objdeploy == nil {
			return fmt.Errorf("objectdeployment not found with: OwnerReferences.UID=%s", pkg.UID)
		}
		objset, err := h.GetObjectSetByOwner(ctx, objdeploy.UID)
		if err != nil {
			return fmt.Errorf("retrieving objectsets: %w", err)
		}
		if objset == nil {
			return fmt.Errorf("objectsets not found with: OwnerReferences.UID=%s", objdeploy.UID)
		}
		fmt.Println(objset)
	case "objectdeployment", "objdeploy":
		objdeploy, err := h.GetObjectDeploymentByName(ctx, h.Name)
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}
		fmt.Println(objdeploy)
		fmt.Println("")
		objset, err := h.GetObjectSetByOwner(ctx, objdeploy.UID)
		fmt.Println(objset)
	default:
		return fmt.Errorf("%w: invalid object. Needs to be one of clusterpackage,clusterobjectdeployment,package,objectdeployment", cmdutil.ErrInvalidArgs)
	}

	return nil
}

func (h *History) GetClusterPackageByName(ctx context.Context, name string) (*corev1alpha1.ClusterPackage, error) {
	var clusterPackageList corev1alpha1.ClusterPackageList

	err := h.client.List(ctx, &clusterPackageList)
	if err != nil {
		return nil, fmt.Errorf("getting packages: %w", err)
	}
	for _, n := range clusterPackageList.Items {
		if name == n.Name {
			return &n, nil
		}
	}
	return nil, nil
}

func (h *History) GetClusterObjectDeploymentByName(ctx context.Context, name string) (*corev1alpha1.ClusterObjectDeployment, error) {
	var clusterObjectDeploymentList corev1alpha1.ClusterObjectDeploymentList

	err := h.client.List(ctx, &clusterObjectDeploymentList)
	if err != nil {
		return nil, fmt.Errorf("getting objectdeployments: %w", err)
	}
	for _, n := range clusterObjectDeploymentList.Items {
		if name == n.Name {
			return &n, nil
		}
	}
	return nil, nil
}

func (h *History) GetClusterObjectDeploymentByOwner(ctx context.Context, ownerUid types.UID) (*corev1alpha1.ClusterObjectDeployment, error) {
	var clusterObjectDeploymentList corev1alpha1.ClusterObjectDeploymentList

	err := h.client.List(ctx, &clusterObjectDeploymentList)
	if err != nil {
		return nil, fmt.Errorf("getting objectdeployments: %w", err)
	}
	for _, n := range clusterObjectDeploymentList.Items {
		for _, owner := range n.OwnerReferences {
			if ownerUid == owner.UID {
				return &n, nil
			}
		}
	}
	return nil, nil
}

func (h *History) GetClusterObjectSetByOwner(ctx context.Context, ownerUid types.UID) (*[]corev1alpha1.ClusterObjectSet, error) {
	var clusterObjectSetList corev1alpha1.ClusterObjectSetList

	err := h.client.List(ctx, &clusterObjectSetList)
	if err != nil {
		return nil, fmt.Errorf("getting objectsets: %w", err)
	}
	var objectSets []corev1alpha1.ClusterObjectSet
	for _, n := range clusterObjectSetList.Items {
		for _, owner := range n.OwnerReferences {
			if ownerUid == owner.UID {
				objectSets = append(objectSets, n)
			}
		}
	}
	return &objectSets, nil
}

func (h *History) GetPackageByName(ctx context.Context, name string) (*corev1alpha1.Package, error) {
	var packageList corev1alpha1.PackageList

	err := h.client.List(ctx, &packageList, client.InNamespace(h.Namespace))
	if err != nil {
		return nil, fmt.Errorf("getting packages: %w", err)
	}
	for _, n := range packageList.Items {
		if name == n.Name {
			return &n, nil
		}
	}
	return nil, nil
}

func (h *History) GetObjectDeploymentByName(ctx context.Context, name string) (*corev1alpha1.ObjectDeployment, error) {
	var objectDeploymentList corev1alpha1.ObjectDeploymentList

	err := h.client.List(ctx, &objectDeploymentList, client.InNamespace(h.Namespace))
	if err != nil {
		return nil, fmt.Errorf("getting objectdeployments: %w", err)
	}
	for _, n := range objectDeploymentList.Items {
		if name == n.Name {
			return &n, nil
		}
	}
	return nil, nil
}

func (h *History) GetObjectDeploymentByOwner(ctx context.Context, ownerUid types.UID) (*corev1alpha1.ObjectDeployment, error) {
	var objectDeploymentList corev1alpha1.ObjectDeploymentList

	err := h.client.List(ctx, &objectDeploymentList, client.InNamespace(h.Namespace))
	if err != nil {
		return nil, fmt.Errorf("getting objectdeployments: %w", err)
	}
	for _, n := range objectDeploymentList.Items {
		for _, owner := range n.OwnerReferences {
			if ownerUid == owner.UID {
				return &n, nil
			}
		}
	}
	return nil, nil
}

func (h *History) GetObjectSetByOwner(ctx context.Context, ownerUid types.UID) (*[]corev1alpha1.ObjectSet, error) {
	var objectSetList corev1alpha1.ObjectSetList

	err := h.client.List(ctx, &objectSetList, client.InNamespace(h.Namespace))
	if err != nil {
		return nil, fmt.Errorf("getting objectsets: %w", err)
	}
	var objectSets []corev1alpha1.ObjectSet
	for _, n := range objectSetList.Items {
		for _, owner := range n.OwnerReferences {
			if ownerUid == owner.UID {
				objectSets = append(objectSets, n)
			}
		}
	}
	return &objectSets, nil
}

func (h *History) CobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   historyUse,
		Short: historyShort,
		Long:  historyLong,
	}
	f := cmd.Flags()
	f.StringVarP(&h.Namespace, "namespace", "n", "", namespaceUse)

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
