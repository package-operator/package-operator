package bootstrap

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func isPKOAvailable(ctx context.Context, c client.Client, pkoNamespace string) (bool, error) {
	deploymentAvailable, err := isPKODeploymentAvailable(ctx, c, pkoNamespace)
	if err != nil || !deploymentAvailable {
		return false, err
	}

	clusterPackageAvailable, err := isPKOClusterPackageAvailable(ctx, c)
	if err != nil {
		return false, err
	}
	return deploymentAvailable && clusterPackageAvailable, nil
}

func isPKOClusterPackageAvailable(ctx context.Context, c client.Client) (bool, error) {
	clusterPackage := &corev1alpha1.ClusterPackage{}
	err := c.Get(ctx, client.ObjectKey{
		Name: packageOperatorClusterPackageName,
	}, clusterPackage)
	if errors.IsNotFound(err) {
		// ClusterPackage does not exist.
		return false, nil
	}
	if err != nil {
		return false, err
	}

	availCond := meta.FindStatusCondition(clusterPackage.Status.Conditions, corev1alpha1.PackageAvailable)
	return availCond.ObservedGeneration == clusterPackage.Generation && availCond.Status == metav1.ConditionTrue, nil
}

func isPKODeploymentAvailable(ctx context.Context, c client.Client, pkoNamespace string) (bool, error) {
	deploy := &appsv1.Deployment{}
	err := c.Get(ctx, client.ObjectKey{
		Name:      packageOperatorDeploymentName,
		Namespace: pkoNamespace,
	}, deploy)
	if errors.IsNotFound(err) {
		// Deployment does not exist.
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Not looking at condition of type `appsv1.DeploymentAvailable` because it can be true when replicas are set to 0.
	if deploy.Status.AvailableReplicas > 0 &&
		deploy.Status.UpdatedReplicas == deploy.Status.AvailableReplicas {
		// Deployment is available -> nothing to do.
		return true, nil
	}

	return false, nil
}
