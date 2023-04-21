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

func GetClusterHistory(ctx context.Context, c client.Client, name string, t interface{}) (*[]v1alpha1.ClusterObjectSet, error) {
	var objDeployInterface interface{}
	var err error

	switch t.(type) {
	case v1alpha1.ClusterPackage:
		pkg, err := GetClusterObjectByName(ctx, c, name, v1alpha1.ClusterPackage{})
		if err != nil {
			return nil, fmt.Errorf("retrieving packages: %w", err)
		} else if pkg == nil {
			return nil, fmt.Errorf("clusterpackages.package-operator.run \"%s\" not found", name)
		}

		objDeployInterface, err = GetClusterObjectByOwner(ctx, c, pkg.(*v1alpha1.ClusterPackage).UID, v1alpha1.ClusterObjectDeployment{})
		if err != nil {
			return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
		}
	case v1alpha1.ClusterObjectDeployment:
		objDeployInterface, err := GetClusterObjectByName(ctx, c, name, v1alpha1.ClusterObjectDeployment{})
		if err != nil {
			return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
		} else if objDeployInterface == nil {
			return nil, fmt.Errorf("clusterobjectdeployments.package-operator.run \"%s\" not found", name)
		}
	default:
		return nil, fmt.Errorf("getting objects: unknown object type")
	}

	objDeploy := objDeployInterface.(*v1alpha1.ClusterObjectDeployment)

	objSets, err := GetClusterObjectByOwner(ctx, c, objDeploy.UID, v1alpha1.ClusterObjectSet{})
	if err != nil {
		return nil, fmt.Errorf("retrieving objectsets: %w", err)
	}
	return objSets.(*[]v1alpha1.ClusterObjectSet), nil
}

func GetNamespacedHistory(ctx context.Context, c client.Client, name string, namespace string, t interface{}) (*[]v1alpha1.ObjectSet, error) {
	var objDeployInterface interface{}
	var err error

	switch t.(type) {
	case v1alpha1.Package:
		pkg, err := GetNamespacedObjectByName(ctx, c, name, namespace, v1alpha1.Package{})
		if err != nil {
			return nil, fmt.Errorf("retrieving packages: %w", err)
		} else if pkg == nil {
			return nil, fmt.Errorf("packages.package-operator.run \"%s\" not found", name)
		}

		objDeployInterface, err = GetNamespacedObjectByOwner(ctx, c, pkg.(*v1alpha1.Package).UID, namespace, v1alpha1.ObjectDeployment{})
		if err != nil {
			return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
		}
	case v1alpha1.ObjectDeployment:
		objDeployInterface, err := GetNamespacedObjectByName(ctx, c, name, namespace, v1alpha1.ObjectDeployment{})
		if err != nil {
			return nil, fmt.Errorf("retrieving objectdeployments: %w", err)
		} else if objDeployInterface == nil {
			return nil, fmt.Errorf("objectdeployments.package-operator.run \"%s\" not found", name)
		}
	default:
		return nil, fmt.Errorf("getting objects: unknown object type")
	}

	objDeploy := objDeployInterface.(*v1alpha1.ObjectDeployment)

	objSets, err := GetNamespacedObjectByOwner(ctx, c, objDeploy.UID, namespace, v1alpha1.ObjectSet{})
	if err != nil {
		return nil, fmt.Errorf("retrieving objectsets: %w", err)
	}
	return objSets.(*[]v1alpha1.ObjectSet), nil
}

func GetClusterObjectByName(ctx context.Context, c client.Client, name string, t interface{}) (interface{}, error) {
	switch t.(type) {
	case v1alpha1.ClusterPackage:
		var list v1alpha1.ClusterPackageList
		err := c.List(ctx, &list)
		if err != nil {
			return nil, fmt.Errorf("getting cluster packages: %w", err)
		}
		for _, n := range list.Items {
			if name == n.Name {
				return &n, nil
			}
		}
	case v1alpha1.ClusterObjectDeployment:
		var list v1alpha1.ClusterObjectDeploymentList
		err := c.List(ctx, &list)
		if err != nil {
			return nil, fmt.Errorf("getting cluster object deployments: %w", err)
		}
		for _, n := range list.Items {
			if name == n.Name {
				return &n, nil
			}
		}
	case v1alpha1.ClusterObjectSet:
		var list v1alpha1.ClusterObjectSetList
		err := c.List(ctx, &list)
		if err != nil {
			return nil, fmt.Errorf("getting cluster object sets: %w", err)
		}
		for _, n := range list.Items {
			if name == n.Name {
				return &n, nil
			}
		}
	default:
		return nil, fmt.Errorf("getting objects: unknown object type")
	}
	return nil, nil
}

func GetNamespacedObjectByName(ctx context.Context, c client.Client, name string, namespace string, t interface{}) (interface{}, error) {
	switch t.(type) {
	case v1alpha1.Package:
		var list v1alpha1.PackageList
		err := c.List(ctx, &list, client.InNamespace(namespace))
		if err != nil {
			return nil, fmt.Errorf("getting packages: %w", err)
		}
		for _, n := range list.Items {
			if name == n.Name {
				return &n, nil
			}
		}
	case v1alpha1.ObjectDeployment:
		var list v1alpha1.ObjectDeploymentList
		err := c.List(ctx, &list, client.InNamespace(namespace))
		if err != nil {
			return nil, fmt.Errorf("getting object deployments: %w", err)
		}
		for _, n := range list.Items {
			if name == n.Name {
				return &n, nil
			}
		}
	case v1alpha1.ObjectSet:
		var list v1alpha1.ObjectSetList
		err := c.List(ctx, &list, client.InNamespace(namespace))
		if err != nil {
			return nil, fmt.Errorf("getting object sets: %w", err)
		}
		for _, n := range list.Items {
			if name == n.Name {
				return &n, nil
			}
		}
	default:
		return nil, fmt.Errorf("getting objects: unknown object type")
	}
	return nil, nil
}

func GetClusterObjectByOwner(ctx context.Context, c client.Client, ownerUid types.UID, t interface{}) (interface{}, error) {
	switch t.(type) {
	case v1alpha1.ClusterPackage:
		var list v1alpha1.ClusterPackageList
		err := c.List(ctx, &list)
		if err != nil {
			return nil, fmt.Errorf("getting cluster packages: %w", err)
		}
		for _, n := range list.Items {
			for _, owner := range n.OwnerReferences {
				if ownerUid == owner.UID {
					return &n, nil
				}
			}
		}
	case v1alpha1.ClusterObjectDeployment:
		var list v1alpha1.ClusterObjectDeploymentList
		err := c.List(ctx, &list)
		if err != nil {
			return nil, fmt.Errorf("getting cluster object deployments: %w", err)
		}
		for _, n := range list.Items {
			for _, owner := range n.OwnerReferences {
				if ownerUid == owner.UID {
					return &n, nil
				}
			}
		}
	case v1alpha1.ClusterObjectSet:
		var list v1alpha1.ClusterObjectSetList
		err := c.List(ctx, &list)
		if err != nil {
			return nil, fmt.Errorf("getting cluster object sets: %w", err)
		}
		var objectSets []v1alpha1.ClusterObjectSet
		for _, n := range list.Items {
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
	default:
		return nil, fmt.Errorf("getting objects: unknown object type")
	}
	return nil, nil
}

func GetNamespacedObjectByOwner(ctx context.Context, c client.Client, ownerUid types.UID, namespace string, t interface{}) (interface{}, error) {
	switch t.(type) {
	case v1alpha1.Package:
		var list v1alpha1.PackageList
		err := c.List(ctx, &list, client.InNamespace(namespace))
		if err != nil {
			return nil, fmt.Errorf("getting packages: %w", err)
		}
		for _, n := range list.Items {
			for _, owner := range n.OwnerReferences {
				if ownerUid == owner.UID {
					return &n, nil
				}
			}
		}
	case v1alpha1.ObjectDeployment:
		var list v1alpha1.ObjectDeploymentList
		err := c.List(ctx, &list, client.InNamespace(namespace))
		if err != nil {
			return nil, fmt.Errorf("getting object deployments: %w", err)
		}
		for _, n := range list.Items {
			for _, owner := range n.OwnerReferences {
				if ownerUid == owner.UID {
					return &n, nil
				}
			}
		}
	case v1alpha1.ObjectSet:
		var list v1alpha1.ObjectSetList
		err := c.List(ctx, &list, client.InNamespace(namespace))
		if err != nil {
			return nil, fmt.Errorf("getting object sets: %w", err)
		}
		var objectSets []v1alpha1.ObjectSet
		for _, n := range list.Items {
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
	default:
		return nil, fmt.Errorf("getting objects: unknown object type")
	}
	return nil, nil
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
