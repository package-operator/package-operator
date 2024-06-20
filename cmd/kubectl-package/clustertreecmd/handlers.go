package clustertreecmd

import (
	"fmt"

	"github.com/disiqueira/gotree"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalcmd "package-operator.run/internal/cmd"
)

func handlePackage(clientL *internalcmd.Client, nsPackage *internalcmd.Package,
	cmd *cobra.Command,
) (string, error) {
	tree := gotree.New(fmt.Sprintf("Package /%s\nnamespace/%s", nsPackage.Name(), nsPackage.Namespace()))
	result, err := clientL.GetObjectset(cmd.Context(), nsPackage.Name(), nsPackage.Namespace())
	if err != nil {
		return "", err
	}

	for _, phase := range result.Spec.Phases {
		treePhase := tree.Add("Phase " + phase.Name)

		for _, obj := range phase.Objects {
			treePhase.Add(
				fmt.Sprintf("%s %s",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}

		for _, obj := range phase.ExternalObjects {
			treePhase.Add(
				fmt.Sprintf("%s %s (EXTERNAL)",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}
	}
	return tree.Print(), nil
}

func handleClusterPackage(clientL *internalcmd.Client, clsPackage *internalcmd.Package,
	cmd *cobra.Command,
) (string, error) {
	tree := gotree.New(fmt.Sprintf("ClusterPackage /%s", clsPackage.Name())) //nolint: perfsprint
	result, err := clientL.GetClusterObjectset(cmd.Context(), clsPackage.Name())
	if err != nil {
		return "", err
	}

	for _, phase := range result.Spec.Phases {
		treePhase := tree.Add("Phase " + phase.Name)

		for _, obj := range phase.Objects {
			treePhase.Add(
				fmt.Sprintf("%s %s",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}

		for _, obj := range phase.ExternalObjects {
			treePhase.Add(
				fmt.Sprintf("%s %s (EXTERNAL)",
					obj.Object.GroupVersionKind(),
					client.ObjectKeyFromObject(&obj.Object)))
		}
	}
	return tree.Print(), nil
}
