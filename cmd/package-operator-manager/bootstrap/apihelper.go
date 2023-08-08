package bootstrap

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isPKOAvailable(ctx context.Context, c client.Client, pkoNamespace string) (bool, error) {
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
	if deploy.Status.AvailableReplicas > 0 {
		// Deployment is available -> nothing to do.
		return true, nil
	}

	return false, nil
}
