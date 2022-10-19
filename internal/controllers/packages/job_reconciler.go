package packages

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type jobReconciler struct {
	scheme           *runtime.Scheme
	newPackage       genericPackageFactory
	client           client.Client
	jobOwnerStrategy ownerStrategy
}

func (r *jobReconciler) Reconcile(
	ctx context.Context, pkg genericPackage,
) (res ctrl.Result, err error) {
	foundJob := &batchv1.Job{}
	desiredJob, err := pkg.RenderPackageLoaderJob()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to render the job resource from packageManifest: %w", err)
	}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(desiredJob), foundJob); err != nil {
		if errors.IsNotFound(err) {
			if err := r.jobOwnerStrategy.SetControllerReference(pkg.ClientObject(), desiredJob); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to set owner reference of the Package on the job '%s': %w", desiredJob.Name, err)
			}
			return ctrl.Result{}, r.client.Create(ctx, desiredJob)
		}
		return ctrl.Result{}, err
	}

	if ComputeHash(foundJob.Spec.Template.Spec, nil) != ComputeHash(desiredJob.Spec.Template.Spec, nil) {
		foundJob.Spec.Template.Spec = desiredJob.Spec.Template.Spec
		return ctrl.Result{}, r.client.Update(ctx, foundJob) // the update should automatically trigger another control loop of the package controller (and hence, this reconciler), next time continuing further execution
	}

	var jobCompleted bool
	for _, cond := range foundJob.Status.Conditions {
		if cond.Type == batchv1.JobComplete &&
			cond.Status == corev1.ConditionTrue {
			jobCompleted = true
			meta.SetStatusCondition(
				pkg.GetConditions(), metav1.Condition{
					Type:               corev1alpha1.PackageUnpacked,
					Status:             metav1.ConditionTrue,
					Reason:             "PackageLoaderSucceeded",
					Message:            "Job to load the package succeeded",
					ObservedGeneration: pkg.ClientObject().GetGeneration(),
				})
			break
		}

		if cond.Type == batchv1.JobFailed &&
			cond.Status == corev1.ConditionTrue {
			jobCompleted = true
			meta.SetStatusCondition(
				pkg.GetConditions(), metav1.Condition{
					Type:               corev1alpha1.PackageUnpacked,
					Status:             metav1.ConditionFalse,
					Reason:             "PackageLoaderFailed",
					Message:            "Job to load the package failed",
					ObservedGeneration: pkg.ClientObject().GetGeneration(),
				})
			return ctrl.Result{}, r.client.Delete(ctx, foundJob)
		}
	}

	if !jobCompleted {
		meta.SetStatusCondition(
			pkg.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.PackageUnpacked,
				Status:             metav1.ConditionFalse,
				Reason:             "PackageLoaderInProgress",
				Message:            "Job to load the package is in progress",
				ObservedGeneration: pkg.ClientObject().GetGeneration(),
			})
	}
	return ctrl.Result{}, nil
}
