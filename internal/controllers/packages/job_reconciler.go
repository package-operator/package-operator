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
	scheme        *runtime.Scheme
	client        client.Client
	ownerStrategy ownerStrategy

	pkoNamespace string
	pkoImage     string
}

func (r *jobReconciler) Reconcile(
	ctx context.Context, pkg genericPackage,
) (res ctrl.Result, err error) {
	foundJob := &batchv1.Job{}

	desiredJob := desiredJob(pkg, r.pkoNamespace, r.pkoImage)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(desiredJob), foundJob); err != nil {
		if errors.IsNotFound(err) {
			if err := r.ownerStrategy.SetControllerReference(pkg.ClientObject(), desiredJob); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to set owner reference of the Package on the job '%s': %w", desiredJob.Name, err)
			}
			return ctrl.Result{}, r.client.Create(ctx, desiredJob)
		}
		return ctrl.Result{}, err
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
			meta.SetStatusCondition(
				pkg.GetConditions(), metav1.Condition{
					Type:               corev1alpha1.PackageUnpacked,
					Status:             metav1.ConditionFalse,
					Reason:             "PackageLoaderFailed",
					Message:            fmt.Sprintf("Job to load the package failed: %s", cond.Message),
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

func desiredJob(pkg genericPackage, pkoNamespace, pkoImage string) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(pkg),
			Namespace: pkoNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: "package-operator",
					InitContainers: []corev1.Container{
						{
							Image: pkoImage,
							Name:  "prepare-loader",
							Command: []string{
								"cp", "-a", "/package-operator-manager", "/loader-bin/package-loader",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "shared-dir",
									MountPath: "/loader-bin",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Image: pkg.GetImage(),
							Name:  "package-loader",
							Command: []string{
								"/.loader-bin/package-loader",
								"-load-package=" + client.ObjectKeyFromObject(pkg.ClientObject()).String(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "shared-dir",
									MountPath: "/.loader-bin",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "shared-dir",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
	return job
}

func jobName(pkg genericPackage) string {
	name := pkg.ClientObject().GetName()
	ns := pkg.ClientObject().GetNamespace()
	if len(ns) == 0 {
		return name + "-loader"
	}
	return fmt.Sprintf("%s-%s-loader", ns, name)
}
