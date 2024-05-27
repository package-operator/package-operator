package clustertreecmd

import (
	"fmt"
	"github.com/disiqueira/gotree"
	"github.com/spf13/cobra"
	internalcmd "package-operator.run/internal/cmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func handlePackage(clientL *internalcmd.Client, Package *internalcmd.Package, cmd *cobra.Command) (string, error) {
	tree := gotree.New(fmt.Sprintf("Package /%s\nnamespace/%s", Package.Name(), Package.Namespace()))
	result, err := clientL.GetObjectset(cmd.Context(), Package.Name(), Package.Namespace())
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
func handleClusterPackage(clientL *internalcmd.Client, Package *internalcmd.Package, cmd *cobra.Command) (string, error) {
	tree := gotree.New(fmt.Sprintf("ClusterPackage /%s", Package.Name()))
	result, err := clientL.GetClusterObjectset(cmd.Context(), Package.Name())
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
