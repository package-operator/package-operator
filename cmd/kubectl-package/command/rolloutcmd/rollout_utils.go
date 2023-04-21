package rolloutcmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"package-operator.run/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetClusterPackageHistory(ctx context.Context, c client.Client, name string) (*[]v1alpha1.ClusterObjectSet, error) {
	pkg, err := GetClusterPackageByName(ctx, c, name)
	if err != nil {
		return nil, fmt.Errorf("retrieving packages: %w", err)
	}
	if pkg == nil {
		return nil, fmt.Errorf("clusterpackages.package-operator.run \"%s\" not found", name)
	}
	objDeploy, err := GetClusterObjectDeploymentByOwner(ctx, c, pkg.UID)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
	}
	objSets, err := GetClusterObjectSetByOwner(ctx, c, objDeploy.UID)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectsets: %w", err)
	}
	return objSets, nil
}

func GetClusterObjectDeploymentHistory(ctx context.Context, c client.Client, name string) (*[]v1alpha1.ClusterObjectSet, error) {
	objDeploy, err := GetClusterObjectDeploymentByName(ctx, c, name)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
	}
	if objDeploy == nil {
		return nil, fmt.Errorf("clusterobjectdeployments.package-operator.run \"%s\" not found", name)
	}
	objSets, err := GetClusterObjectSetByOwner(ctx, c, objDeploy.UID)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectsets: %w", err)
	}
	return objSets, nil
}

func GetPackageHistory(ctx context.Context, c client.Client, name string, namespace string) (*[]v1alpha1.ObjectSet, error) {
	pkg, err := GetPackageByName(ctx, c, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("retrieving packages: %w", err)
	}
	if pkg == nil {
		return nil, fmt.Errorf("packages.package-operator.run \"%s\" not found", name)
	}
	objDeploy, err := GetObjectDeploymentByOwner(ctx, c, pkg.UID, namespace)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
	}
	objSets, err := GetObjectSetByOwner(ctx, c, objDeploy.UID, namespace)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectsets: %w", err)
	}
	return objSets, nil
}

func GetObjectDeploymentHistory(ctx context.Context, c client.Client, name string, namespace string) (*[]v1alpha1.ObjectSet, error) {
	objDeploy, err := GetObjectDeploymentByName(ctx, c, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
	}
	if objDeploy == nil {
		return nil, fmt.Errorf("objectdeployments.package-operator.run \"%s\" not found", name)
	}
	objSets, err := GetObjectSetByOwner(ctx, c, objDeploy.UID, namespace)
	if err != nil {
		return nil, fmt.Errorf("retrieving objectsets: %w", err)
	}
	return objSets, nil
}

func GetClusterPackageByName(ctx context.Context, c client.Client, name string) (*v1alpha1.ClusterPackage, error) {
	var clusterPackageList v1alpha1.ClusterPackageList

	err := c.List(ctx, &clusterPackageList)
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

func GetClusterObjectDeploymentByName(ctx context.Context, c client.Client, name string) (*v1alpha1.ClusterObjectDeployment, error) {
	var clusterObjectDeploymentList v1alpha1.ClusterObjectDeploymentList

	err := c.List(ctx, &clusterObjectDeploymentList)
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

func GetClusterObjectDeploymentByOwner(ctx context.Context, c client.Client, ownerUid types.UID) (*v1alpha1.ClusterObjectDeployment, error) {
	var clusterObjectDeploymentList v1alpha1.ClusterObjectDeploymentList

	err := c.List(ctx, &clusterObjectDeploymentList)
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

func GetClusterObjectSetByOwner(ctx context.Context, c client.Client, ownerUid types.UID) (*[]v1alpha1.ClusterObjectSet, error) {
	var clusterObjectSetList v1alpha1.ClusterObjectSetList

	err := c.List(ctx, &clusterObjectSetList)
	if err != nil {
		return nil, fmt.Errorf("getting objectsets: %w", err)
	}
	var objectSets []v1alpha1.ClusterObjectSet
	for _, n := range clusterObjectSetList.Items {
		for _, owner := range n.OwnerReferences {
			if ownerUid == owner.UID {
				objectSets = append(objectSets, n)
			}
		}
	}
	return &objectSets, nil
}

func GetPackageByName(ctx context.Context, c client.Client, name string, namespace string) (*v1alpha1.Package, error) {
	var packageList v1alpha1.PackageList

	err := c.List(ctx, &packageList, client.InNamespace(namespace))
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

func GetObjectDeploymentByName(ctx context.Context, c client.Client, name string, namespace string) (*v1alpha1.ObjectDeployment, error) {
	var objectDeploymentList v1alpha1.ObjectDeploymentList

	err := c.List(ctx, &objectDeploymentList, client.InNamespace(namespace))
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

func GetObjectDeploymentByOwner(ctx context.Context, c client.Client, ownerUid types.UID, namespace string) (*v1alpha1.ObjectDeployment, error) {
	var objectDeploymentList v1alpha1.ObjectDeploymentList

	err := c.List(ctx, &objectDeploymentList, client.InNamespace(namespace))
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

func GetObjectSetByOwner(ctx context.Context, c client.Client, ownerUid types.UID, namespace string) (*[]v1alpha1.ObjectSet, error) {
	var objectSetList v1alpha1.ObjectSetList

	err := c.List(ctx, &objectSetList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("getting objectsets: %w", err)
	}
	var objectSets []v1alpha1.ObjectSet
	for _, n := range objectSetList.Items {
		for _, owner := range n.OwnerReferences {
			if ownerUid == owner.UID {
				objectSets = append(objectSets, n)
			}
		}
	}
	sort.Slice(objectSets, func(i, j int) bool {
		return objectSets[i].Status.Revision < objectSets[j].Status.Revision
	})

	return &objectSets, nil
}

func GetNamespacedRevision(objectSets *[]v1alpha1.ObjectSet, revision int64) (*v1alpha1.ObjectSet, error) {
	for _, objectSet := range *objectSets {
		if revision == objectSet.Status.Revision {
			return &objectSet, nil
		}
	}
	return nil, fmt.Errorf("unable to find the specified revision", revision)
}

func GetClusterRevision(objectSets *[]v1alpha1.ClusterObjectSet, revision int64) (*v1alpha1.ClusterObjectSet, error) {
	for _, objectSet := range *objectSets {
		if revision == objectSet.Status.Revision {
			return &objectSet, nil
		}
	}
	return nil, fmt.Errorf("unable to find the specified revision")
}

func PrintHistory(object string, name string, objectSets *[]v1alpha1.ObjectSet, output string) error {
	switch strings.ToLower(output) {
	case "":
		fmt.Printf("%s/%s\n", object, name)
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
		fmt.Fprintln(w, "REVISION\tSTATUS\tROLLOUT-SUCCESS\tCHANGE-CAUSE\t")
		for _, os := range *objectSets {
			var changeCause, rolloutSuccess string
			if os.ObjectMeta.Annotations["kubernetes.io/change-cause"] == "" {
				changeCause = "<none>"
			} else {
				changeCause = os.ObjectMeta.Annotations["kubernetes.io/change-cause"]
			}
			for _, condifion := range os.Status.Conditions {
				if condifion.Reason == "RolloutSuccess" {
					rolloutSuccess = string(condifion.Status)
				}
			}
			if rolloutSuccess == "" {
				rolloutSuccess = "False"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t\n", os.Status.Revision, os.Status.Phase, rolloutSuccess, changeCause)
		}
		w.Flush()
	case "json":
		json, _ := json.MarshalIndent(*objectSets, "", "    ")
		fmt.Println(string(json))
	case "yaml":
		yaml, _ := yaml.Marshal(*objectSets)
		fmt.Println(string(yaml))
	case "name":
		for _, os := range *objectSets {
			fmt.Printf("%s/%s\n", object, os.ObjectMeta.Name)
		}
	default:
		return fmt.Errorf("unable match output format, allowed formats are: json,yaml,name")

	}

	return nil
}

func PrintClusterHistory(object string, name string, objectSets *[]v1alpha1.ClusterObjectSet, output string) error {
	switch strings.ToLower(output) {
	case "":
		fmt.Printf("%s/%s\n", object, name)
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
		fmt.Fprintln(w, "REVISION\tSTATUS\tROLLOUT-SUCCESS\tCHANGE-CAUSE\t")
		for _, os := range *objectSets {
			var changeCause, rolloutSuccess string
			if os.ObjectMeta.Annotations["kubernetes.io/change-cause"] == "" {
				changeCause = "<none>"
			} else {
				changeCause = os.ObjectMeta.Annotations["kubernetes.io/change-cause"]
			}
			for _, condifion := range os.Status.Conditions {
				if condifion.Reason == "RolloutSuccess" {
					rolloutSuccess = string(condifion.Status)
				}
			}
			if rolloutSuccess == "" {
				rolloutSuccess = "False"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t\n", os.Status.Revision, os.Status.Phase, rolloutSuccess, changeCause)
		}
		w.Flush()
	case "json":
		json, _ := json.MarshalIndent(*objectSets, "", "    ")
		fmt.Println(string(json))
	case "yaml":
		yaml, _ := yaml.Marshal(*objectSets)
		fmt.Println(string(yaml))
	case "name":
		for _, os := range *objectSets {
			fmt.Printf("%s/%s\n", object, os.ObjectMeta.Name)
		}
	default:
		return fmt.Errorf("unable match output format, allowed formats are: json,yaml,name")
	}

	return nil
}

func PrintRevision(object string, name string, revision *v1alpha1.ObjectSet, output string) error {
	switch strings.ToLower(output) {
	case "":
		rev := *revision

		output := fmt.Sprintf("%s/%s with revision #%d\nObjectSetSpec:\n", object, name, revision.Status.Revision)
		yaml, _ := yaml.Marshal(rev.Spec)
		scanner := bufio.NewScanner(strings.NewReader(string(yaml)))
		for scanner.Scan() {
			output += "  " + scanner.Text() + "\n"
		}

		fmt.Println(output)
	case "json":
		json, _ := json.MarshalIndent(*revision, "", "    ")
		fmt.Println(string(json))
	case "yaml":
		yaml, _ := yaml.Marshal(*revision)
		fmt.Println(string(yaml))
	case "name":
		rev := *revision
		fmt.Printf("%s/%s\n", object, rev.ObjectMeta.Name)
	default:
		return fmt.Errorf("unable match output format, allowed formats are: json,yaml,name")
	}

	return nil
}

func PrintClusterRevision(object string, name string, revision *v1alpha1.ClusterObjectSet, output string) error {
	switch strings.ToLower(output) {
	case "":
		rev := *revision

		fmt.Printf("%s/%s with revision #%d\n", object, name, revision.Status.Revision)
		fmt.Println(rev)
	case "json":
		json, _ := json.MarshalIndent(*revision, "", "    ")
		fmt.Println(string(json))
	case "yaml":
		yaml, _ := yaml.Marshal(*revision)
		fmt.Println(string(yaml))
	case "name":
		rev := *revision
		fmt.Printf("%s/%s\n", object, rev.ObjectMeta.Name)
	default:
		return fmt.Errorf("unable match output format, allowed formats are: json,yaml,name")
	}

	return nil

}
