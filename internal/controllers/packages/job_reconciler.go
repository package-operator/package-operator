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
	"package-operator.run/package-operator/internal/controllers"
)

const (
	// Hash used to determine if the job is still up-to-date.
	packageSpecHashAnnotation = "package-operator.run/package-spec-hash"
	// Name of the Package object the loader job belongs to.
	packageNameLabel = "package-operator.run/pkg-name"
	// Namespace of the Package object the loader job belongs to.
	packageNamespaceLabel = "package-operator.run/pkg-namespace"
)

type jobReconciler struct {
	scheme        *runtime.Scheme
	client        client.Client
	ownerStrategy ownerStrategy

	pkoNamespace string
	pkoImage     string
}

func newJobReconciler(
	scheme *runtime.Scheme,
	client client.Client,
	ownerStrategy ownerStrategy,
	pkoNamespace string,
	pkoImage string,
) *jobReconciler {
	return &jobReconciler{
		scheme:        scheme,
		client:        client,
		ownerStrategy: ownerStrategy,
		pkoNamespace:  pkoNamespace,
		pkoImage:      pkoImage,
	}
}

func (r *jobReconciler) Reconcile(
	ctx context.Context, pkg genericPackage,
) (res ctrl.Result, err error) {
	job, err := r.ensureUnpackJob(ctx, pkg)
	if err != nil {
		return res, fmt.Errorf("ensure unpack job: %w", err)
	}

	var jobCompleted bool
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete &&
			cond.Status == corev1.ConditionTrue {
			jobCompleted = true
			meta.SetStatusCondition(
				pkg.GetConditions(), metav1.Condition{
					Type:               corev1alpha1.PackageUnpacked,
					Status:             metav1.ConditionTrue,
					Reason:             "UnpackSuccess",
					Message:            "Unpack job succeeded",
					ObservedGeneration: pkg.ClientObject().GetGeneration(),
				})
			continue
		}

		if cond.Type == batchv1.JobFailed &&
			cond.Status == corev1.ConditionTrue {
			jobCompleted = true
			meta.SetStatusCondition(
				pkg.GetConditions(), metav1.Condition{
					Type:               corev1alpha1.PackageUnpacked,
					Status:             metav1.ConditionFalse,
					Reason:             "UnpackFailure",
					Message:            "Unpack job failed",
					ObservedGeneration: pkg.ClientObject().GetGeneration(),
				})
			if err := r.client.Delete(ctx, job); err != nil {
				return res, fmt.Errorf("deleting failed job: %w", err)
			}
		}
	}

	if !jobCompleted {
		meta.SetStatusCondition(
			pkg.GetConditions(), metav1.Condition{
				Type:               corev1alpha1.PackageUnpacked,
				Status:             metav1.ConditionFalse,
				Reason:             "Unpacking",
				Message:            "Unpack job in progress",
				ObservedGeneration: pkg.ClientObject().GetGeneration(),
			})
	}
	return ctrl.Result{}, nil
}

func (r *jobReconciler) ensureUnpackJob(
	ctx context.Context, packageObj genericPackage,
) (*batchv1.Job, error) {
	desiredJob := desiredJob(packageObj, r.pkoNamespace, r.pkoImage)
	if err := r.ownerStrategy.SetControllerReference(
		packageObj.ClientObject(), desiredJob); err != nil {
		return nil, fmt.Errorf("set controller reference: %w", err)
	}

	existingJob := &batchv1.Job{}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(desiredJob), existingJob); err != nil && errors.IsNotFound(err) {
		if err := r.client.Create(ctx, desiredJob); err != nil {
			return nil, fmt.Errorf("creating Job: %w", err)
		}
		return desiredJob, nil
	} else if err != nil {
		return nil, fmt.Errorf("getting Job: %w", err)
	}

	if existingJob.Annotations == nil ||
		existingJob.Annotations[packageSpecHashAnnotation] !=
			desiredJob.Annotations[packageSpecHashAnnotation] {
		// re-create job
		if err := r.client.Delete(ctx, existingJob); err != nil {
			return nil, fmt.Errorf("deleting outdated Job: %w", err)
		}
		if err := r.client.Create(ctx, desiredJob); err != nil {
			return nil, fmt.Errorf("creating Job: %w", err)
		}
		return desiredJob, nil
	}

	return existingJob, nil
}

func desiredJob(pkg genericPackage, pkoNamespace, pkoImage string) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(pkg),
			Namespace: pkoNamespace,
			Annotations: map[string]string{
				packageSpecHashAnnotation: pkg.GetSpecHash(),
			},
			Labels: map[string]string{
				controllers.DynamicCacheLabel: "True",
				packageNameLabel:              pkg.ClientObject().GetName(),
				packageNamespaceLabel:         pkg.ClientObject().GetNamespace(),
			},
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
							Args: []string{
								"-copy-to=/loader-bin/package-loader",
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
	return "loader-" + string(pkg.ClientObject().GetUID())
}
