package adapters

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/utils"
)

var (
	packageGVK        = corev1alpha1.GroupVersion.WithKind("Package")
	clusterPackageGVK = corev1alpha1.GroupVersion.WithKind("ClusterPackage")
)

// PackageAccessor is an adapter interface to access a Package.
//
// Reason for this interface is that it allows accessing an Package in two scopes:
// The regular Package and the ClusterPackage.
type PackageAccessor interface {
	ClientObject() client.Object

	GetSpecComponent() string
	GetSpecConditions() *[]metav1.Condition
	GetSpecImage() string
	GetSpecHash(packageHashModifier *int32) string
	GetSpecPaused() bool
	SetSpecPaused(paused bool)
	GetSpecTemplateContext() manifests.TemplateContext

	GetStatusRevision() int64
	SetStatusRevision(rev int64)
	GetStatusUnpackedHash() string
	SetStatusUnpackedHash(hash string)
}

type GenericPackageFactory func(scheme *runtime.Scheme) PackageAccessor

func NewGenericPackage(scheme *runtime.Scheme) PackageAccessor {
	obj, err := scheme.New(packageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericPackage{Package: *obj.(*corev1alpha1.Package)}
}

func NewGenericClusterPackage(scheme *runtime.Scheme) PackageAccessor {
	obj, err := scheme.New(clusterPackageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterPackage{ClusterPackage: *obj.(*corev1alpha1.ClusterPackage)}
}

var (
	_ PackageAccessor = (*GenericPackage)(nil)
	_ PackageAccessor = (*GenericClusterPackage)(nil)
)

type GenericPackage struct {
	corev1alpha1.Package
}

func (a *GenericPackage) ClientObject() client.Object {
	return &a.Package
}

func (a *GenericPackage) GetSpecComponent() string {
	return a.Spec.Component
}

func (a *GenericPackage) GetSpecConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericPackage) GetSpecImage() string {
	return a.Spec.Image
}

func (a *GenericPackage) GetSpecHash(packageHashModifier *int32) string {
	return utils.ComputeSHA256Hash(a.Spec, packageHashModifier)
}

func (a *GenericPackage) SetStatusUnpackedHash(hash string) {
	a.Status.UnpackedHash = hash
}

func (a *GenericPackage) GetStatusUnpackedHash() string {
	return a.Status.UnpackedHash
}

func (a *GenericPackage) SetStatusRevision(rev int64) {
	a.Status.Revision = rev
}

func (a *GenericPackage) GetStatusRevision() int64 {
	return a.Status.Revision
}

func (a *GenericPackage) GetSpecTemplateContext() manifests.TemplateContext {
	return manifests.TemplateContext{
		Package: manifests.TemplateContextPackage{
			TemplateContextObjectMeta: templateContextObjectMetaFromObjectMeta(a.ObjectMeta),
			Image:                     a.Spec.Image,
		},
		Config: a.Spec.Config,
	}
}

func (a *GenericPackage) GetSpecPaused() bool {
	return a.Spec.Paused
}

func (a *GenericPackage) SetSpecPaused(paused bool) {
	a.Spec.Paused = paused
}

type GenericClusterPackage struct {
	corev1alpha1.ClusterPackage
}

func (a *GenericClusterPackage) ClientObject() client.Object {
	return &a.ClusterPackage
}

func (a *GenericClusterPackage) GetSpecComponent() string {
	return a.Spec.Component
}

func (a *GenericClusterPackage) GetSpecConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterPackage) GetSpecImage() string {
	return a.Spec.Image
}

func (a *GenericClusterPackage) GetSpecHash(packageHashModifier *int32) string {
	return utils.ComputeSHA256Hash(a.Spec, packageHashModifier)
}

func (a *GenericClusterPackage) SetStatusRevision(rev int64) {
	a.Status.Revision = rev
}

func (a *GenericClusterPackage) GetStatusRevision() int64 {
	return a.Status.Revision
}

func (a *GenericClusterPackage) GetSpecTemplateContext() manifests.TemplateContext {
	return manifests.TemplateContext{
		Package: manifests.TemplateContextPackage{
			TemplateContextObjectMeta: templateContextObjectMetaFromObjectMeta(a.ObjectMeta),
			Image:                     a.Spec.Image,
		},
		Config: a.Spec.Config,
	}
}

func (a *GenericClusterPackage) SetStatusUnpackedHash(hash string) {
	a.Status.UnpackedHash = hash
}

func (a *GenericClusterPackage) GetStatusUnpackedHash() string {
	return a.Status.UnpackedHash
}

func (a *GenericClusterPackage) GetSpecPaused() bool {
	return a.Spec.Paused
}

func (a *GenericClusterPackage) SetSpecPaused(paused bool) {
	a.Spec.Paused = paused
}

func templateContextObjectMetaFromObjectMeta(om metav1.ObjectMeta) manifests.TemplateContextObjectMeta {
	return manifests.TemplateContextObjectMeta{
		Name:        om.Name,
		Namespace:   om.Namespace,
		Labels:      om.Labels,
		Annotations: om.Annotations,
	}
}
