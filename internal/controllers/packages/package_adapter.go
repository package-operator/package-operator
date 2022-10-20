package packages

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type genericPackage interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	RenderPackageLoaderJob() *batchv1.Job
	GetImage() string
}

type genericPackageFactory func(scheme *runtime.Scheme) genericPackage
type genericObjectDeploymentFactory func(scheme *runtime.Scheme) genericObjectDeployment

var (
	packageGVK        = corev1alpha1.GroupVersion.WithKind("Package")
	clusterPackageGVK = corev1alpha1.GroupVersion.WithKind("ClusterPackage")
)

func newGenericPackage(scheme *runtime.Scheme) genericPackage {
	obj, err := scheme.New(packageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericPackage{
		Package: *obj.(*corev1alpha1.Package)}
}

func newGenericClusterPackage(scheme *runtime.Scheme) genericPackage {
	obj, err := scheme.New(clusterPackageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterPackage{
		ClusterPackage: *obj.(*corev1alpha1.ClusterPackage)}
}

func newGenericClusterObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(clusterObjectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectDeployment{
		ClusterObjectDeployment: *obj.(*corev1alpha1.ClusterObjectDeployment)}
}

func newGenericObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(objectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectDeployment{
		ObjectDeployment: *obj.(*corev1alpha1.ObjectDeployment)}
}

var (
	_ genericPackage = (*GenericPackage)(nil)
	_ genericPackage = (*GenericClusterPackage)(nil)
)

type GenericPackage struct {
	corev1alpha1.Package
}

func (a *GenericPackage) ClientObject() client.Object {
	return &a.Package
}

func (a *GenericPackage) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericPackage) UpdatePhase() {
	if meta.IsStatusConditionFalse(
		a.Status.Conditions,
		corev1alpha1.PackageUnpacked,
	) {
		a.Status.Phase = corev1alpha1.PackagePhaseUnpacking
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.PackageProgressing,
	) {
		a.Status.Phase = corev1alpha1.PackagePhaseProgressing
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.PackageAvailable,
	) {
		a.Status.Phase = corev1alpha1.PackagePhaseAvailable
		return
	}

	a.Status.Phase = corev1alpha1.PackagePhaseNotReady
}

type GenericClusterPackage struct {
	corev1alpha1.ClusterPackage
}

func (a *GenericClusterPackage) ClientObject() client.Object {
	return &a.ClusterPackage
}

func (a *GenericClusterPackage) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericPackage) RenderPackageLoaderJob() *batchv1.Job {
	packageName, packageNamespace, packageImage := a.Package.Name, a.Package.Namespace, a.Package.Spec.Image
	jobName := fmt.Sprintf("job-%s", packageName)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: "package-operator-system", // TODO: sort out all the blockers hurdling us to spin up this job in the `packageNamespace`
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: "package-operator",
					InitContainers: []corev1.Container{
						{
							Image: "quay.io/mtsre/package-loader:test",
							Name:  "prepare-loader",
							Command: []string{
								"cp", "/package-loader", "/loader-bin/package-loader",
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
							Image: packageImage,
							Name:  "package-loader",
							Command: []string{
								"/.loader-bin/package-loader",
								"-scope", "namespace",
								"-package-dir=/package",
								"-package-name", packageName,
								"-package-namespace", packageNamespace,
								"-debug", "true",
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

func (a *GenericClusterPackage) RenderPackageLoaderJob() *batchv1.Job {
	packageName, packageImage := a.ClusterPackage.Name, a.ClusterPackage.Spec.Image
	jobName := fmt.Sprintf("job-%s", packageName)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: "package-operator-system",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					ServiceAccountName: "package-operator",
					InitContainers: []corev1.Container{
						{
							Image: "quay.io/mtsre/package-loader:test",
							Name:  "prepare-loader",
							Command: []string{
								"cp", "/package-loader", "/loader-bin/package-loader",
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
							Image: packageImage,
							Name:  "package-loader",
							Command: []string{
								"/.loader-bin/package-loader",
								"-scope", "cluster",
								"-package-dir=/package",
								"-package-name", packageName,
								"-debug", "true",
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

func (a *GenericClusterPackage) UpdatePhase() {
	if meta.IsStatusConditionFalse(
		a.Status.Conditions,
		corev1alpha1.PackageUnpacked,
	) {
		a.Status.Phase = corev1alpha1.PackagePhaseUnpacking
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.PackageProgressing,
	) {
		a.Status.Phase = corev1alpha1.PackagePhaseProgressing
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		corev1alpha1.PackageAvailable,
	) {
		a.Status.Phase = corev1alpha1.PackagePhaseAvailable
		return
	}

	a.Status.Phase = corev1alpha1.PackagePhaseNotReady
}

func (a *GenericPackage) GetImage() string {
	return a.Spec.Image
}

func (a *GenericClusterPackage) GetImage() string {
	return a.Spec.Image
}

type genericObjectDeployment interface {
	ClientObject() client.Object
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
	SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase)
	GetConditions() []metav1.Condition
	GetObjectMeta() metav1.ObjectMeta
	SetObjectMeta(metav1.ObjectMeta)
}

var (
	_ genericObjectDeployment = (*GenericObjectDeployment)(nil)
	_ genericObjectDeployment = (*GenericClusterObjectDeployment)(nil)
)

type GenericObjectDeployment struct {
	corev1alpha1.ObjectDeployment
}

func (a *GenericObjectDeployment) ClientObject() client.Object {
	return &a.ObjectDeployment
}

func (a *GenericObjectDeployment) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Template.Spec.Phases
}

func (a *GenericObjectDeployment) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Template.Spec.Phases = phases
}

func (a *GenericObjectDeployment) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericObjectDeployment) GetObjectMeta() metav1.ObjectMeta {
	return a.ObjectMeta
}

func (a *GenericObjectDeployment) SetObjectMeta(m metav1.ObjectMeta) {
	a.ObjectMeta = m
}

type GenericClusterObjectDeployment struct {
	corev1alpha1.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) GetPhases() []corev1alpha1.ObjectSetTemplatePhase {
	return a.Spec.Template.Spec.Phases
}

func (a *GenericClusterObjectDeployment) SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase) {
	a.Spec.Template.Spec.Phases = phases
}

func (a *GenericClusterObjectDeployment) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericClusterObjectDeployment) GetObjectMeta() metav1.ObjectMeta {
	return a.ObjectMeta
}

func (a *GenericClusterObjectDeployment) SetObjectMeta(m metav1.ObjectMeta) {
	a.ObjectMeta = m
}

var (
	objectDeploymentGVK        = corev1alpha1.GroupVersion.WithKind("ObjectDeployment")
	clusterObjectDeploymentGVK = corev1alpha1.GroupVersion.WithKind("ClusterObjectDeployment")
)
