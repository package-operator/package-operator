package packages

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type Finalizer string
type Cleaner func(c client.Client, obj genericPackage) error

const ObjectDeploymentFinalizer Finalizer = "package-operator.run/object-deployment"
const JobFinalizer Finalizer = "package-operator.run/job"

var FinalizersToCleaners = map[Finalizer]Cleaner{
	ObjectDeploymentFinalizer: func(c client.Client, pkg genericPackage) error {
		objectDeploymentName, objectDeploymentNamespace := pkg.ClientObject().GetName(), pkg.ClientObject().GetNamespace()
		objectDeployment := &corev1alpha1.ObjectDeployment{}
		if err := c.Get(context.TODO(), types.NamespacedName{Namespace: objectDeploymentNamespace, Name: objectDeploymentName}, objectDeployment); err != nil {
			return client.IgnoreNotFound(err)
		}
		if err := c.Delete(context.TODO(), objectDeployment); err != nil {
			return fmt.Errorf("failed to delete the ObjectDeployment: %w", err)
		}
		return nil
	},
	JobFinalizer: func(c client.Client, pkg genericPackage) error {
		jobName, jobNamespace := "job-"+pkg.ClientObject().GetName(), "package-operator-system"
		job := &batchv1.Job{}
		if err := c.Get(context.TODO(), types.NamespacedName{Namespace: jobNamespace, Name: jobName}, job); err != nil {
			return client.IgnoreNotFound(err)
		}
		if err := c.Delete(context.TODO(), job); err != nil {
			return fmt.Errorf("failed to delete the Job: %w", err)
		}
		return nil
	},
}

func packageFinalizers() []string {
	res := []string{}
	for finalizer := range FinalizersToCleaners {
		res = append(res, string(finalizer))
	}
	return res
}
