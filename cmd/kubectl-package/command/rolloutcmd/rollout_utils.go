package rolloutcmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetClusterPackageHistory(ctx context.Context, c client.Client, name string) (*[]corev1alpha1.ClusterObjectSet, error) {
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

func GetClusterObjectDeploymentHistory(ctx context.Context, c client.Client, name string) (*[]corev1alpha1.ClusterObjectSet, error) {
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

func GetPackageHistory(ctx context.Context, c client.Client, name string, namespace string) (*[]corev1alpha1.ObjectSet, error) {
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

func GetObjectDeploymentHistory(ctx context.Context, c client.Client, name string, namespace string) (*[]corev1alpha1.ObjectSet, error) {
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

func GetClusterPackageByName(ctx context.Context, c client.Client, name string) (*corev1alpha1.ClusterPackage, error) {
	var clusterPackageList corev1alpha1.ClusterPackageList

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

func GetClusterObjectDeploymentByName(ctx context.Context, c client.Client, name string) (*corev1alpha1.ClusterObjectDeployment, error) {
	var clusterObjectDeploymentList corev1alpha1.ClusterObjectDeploymentList

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

func GetClusterObjectDeploymentByOwner(ctx context.Context, c client.Client, ownerUid types.UID) (*corev1alpha1.ClusterObjectDeployment, error) {
	var clusterObjectDeploymentList corev1alpha1.ClusterObjectDeploymentList

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

func GetClusterObjectSetByOwner(ctx context.Context, c client.Client, ownerUid types.UID) (*[]corev1alpha1.ClusterObjectSet, error) {
	var clusterObjectSetList corev1alpha1.ClusterObjectSetList

	err := c.List(ctx, &clusterObjectSetList)
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

func GetPackageByName(ctx context.Context, c client.Client, name string, namespace string) (*corev1alpha1.Package, error) {
	var packageList corev1alpha1.PackageList

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

func GetObjectDeploymentByName(ctx context.Context, c client.Client, name string, namespace string) (*corev1alpha1.ObjectDeployment, error) {
	var objectDeploymentList corev1alpha1.ObjectDeploymentList

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

func GetObjectDeploymentByOwner(ctx context.Context, c client.Client, ownerUid types.UID, namespace string) (*corev1alpha1.ObjectDeployment, error) {
	var objectDeploymentList corev1alpha1.ObjectDeploymentList

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

func GetObjectSetByOwner(ctx context.Context, c client.Client, ownerUid types.UID, namespace string) (*[]corev1alpha1.ObjectSet, error) {
	var objectSetList corev1alpha1.ObjectSetList

	err := c.List(ctx, &objectSetList, client.InNamespace(namespace))
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
	sort.Slice(objectSets, func(i, j int) bool {
		return objectSets[i].Status.Revision < objectSets[j].Status.Revision
	})

	return &objectSets, nil
}

func HistoryResults(object string, name string, objectSets *[]corev1alpha1.ObjectSet) error {
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

	return nil
}

func HistoryClusterResults(object string, name string, objectSets *[]corev1alpha1.ClusterObjectSet) error {
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

	return nil
}
