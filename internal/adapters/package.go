package adapters

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/utils"
)

type GenericPackageAccessor interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetImage() string
	GetSpecHash(packageHashModifier *int32) string
	GetUnpackedHash() string
	SetUnpackedHash(hash string)
	setStatusPhase(phase corev1alpha1.PackageStatusPhase)
	TemplateContext() manifestsv1alpha1.TemplateContext
	SetStatusRevision(rev int64)
	GetStatusRevision() int64
}

type GenericPackageFactory func(scheme *runtime.Scheme) GenericPackageAccessor

var (
	packageGVK        = corev1alpha1.GroupVersion.WithKind("Package")
	clusterPackageGVK = corev1alpha1.GroupVersion.WithKind("ClusterPackage")
)

func NewGenericPackage(scheme *runtime.Scheme) GenericPackageAccessor {
	obj, err := scheme.New(packageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericPackage{
		Package: *obj.(*corev1alpha1.Package),
	}
}

func NewGenericClusterPackage(scheme *runtime.Scheme) GenericPackageAccessor {
	obj, err := scheme.New(clusterPackageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterPackage{
		ClusterPackage: *obj.(*corev1alpha1.ClusterPackage),
	}
}

var (
	_ GenericPackageAccessor = (*GenericPackage)(nil)
	_ GenericPackageAccessor = (*GenericClusterPackage)(nil)
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
	updatePackagePhase(a)
}

func (a *GenericPackage) GetImage() string {
	return a.Spec.Image
}

func (a *GenericPackage) GetSpecHash(packageHashModifier *int32) string {
	return utils.ComputeSHA256Hash(a.Spec, packageHashModifier)
}

func (a *GenericPackage) SetUnpackedHash(hash string) {
	a.Status.UnpackedHash = hash
}

func (a *GenericPackage) GetUnpackedHash() string {
	return a.Status.UnpackedHash
}

func (a *GenericPackage) SetStatusRevision(rev int64) {
	a.Status.Revision = rev
}

func (a *GenericPackage) GetStatusRevision() int64 {
	return a.Status.Revision
}

func (a *GenericPackage) setStatusPhase(phase corev1alpha1.PackageStatusPhase) {
	a.Status.Phase = phase
}

func (a *GenericPackage) TemplateContext() manifestsv1alpha1.TemplateContext {
	return manifestsv1alpha1.TemplateContext{
		Package: manifestsv1alpha1.TemplateContextPackage{
			TemplateContextObjectMeta: templateContextObjectMetaFromObjectMeta(a.ObjectMeta),
		},
		Config: a.Package.Spec.Config,
	}
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

func (a *GenericClusterPackage) UpdatePhase() {
	updatePackagePhase(a)
}

func (a *GenericClusterPackage) GetImage() string {
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

func (a *GenericClusterPackage) setStatusPhase(phase corev1alpha1.PackageStatusPhase) {
	a.Status.Phase = phase
}

func (a *GenericClusterPackage) TemplateContext() manifestsv1alpha1.TemplateContext {
	return manifestsv1alpha1.TemplateContext{
		Package: manifestsv1alpha1.TemplateContextPackage{
			TemplateContextObjectMeta: templateContextObjectMetaFromObjectMeta(a.ObjectMeta),
		},
		Config: a.Spec.Config,
	}
}

func (a *GenericClusterPackage) SetUnpackedHash(hash string) {
	a.Status.UnpackedHash = hash
}

func (a *GenericClusterPackage) GetUnpackedHash() string {
	return a.Status.UnpackedHash
}

func updatePackagePhase(pkg GenericPackageAccessor) {
	if meta.IsStatusConditionTrue(*pkg.GetConditions(), corev1alpha1.PackageInvalid) {
		pkg.setStatusPhase(corev1alpha1.PackagePhaseInvalid)
		return
	}

	unpackCond := meta.FindStatusCondition(*pkg.GetConditions(), corev1alpha1.PackageUnpacked)
	if unpackCond == nil {
		pkg.setStatusPhase(corev1alpha1.PackagePhaseUnpacking)
		return
	}

	if meta.IsStatusConditionTrue(
		*pkg.GetConditions(),
		corev1alpha1.PackageProgressing,
	) {
		pkg.setStatusPhase(corev1alpha1.PackagePhaseProgressing)
		return
	}

	if meta.IsStatusConditionTrue(
		*pkg.GetConditions(),
		corev1alpha1.PackageAvailable,
	) {
		pkg.setStatusPhase(corev1alpha1.PackagePhaseAvailable)
		return
	}

	pkg.setStatusPhase(corev1alpha1.PackagePhaseNotReady)
}

func templateContextObjectMetaFromObjectMeta(om metav1.ObjectMeta) manifestsv1alpha1.TemplateContextObjectMeta {
	return manifestsv1alpha1.TemplateContextObjectMeta{
		Name:        om.Name,
		Namespace:   om.Namespace,
		Labels:      om.Labels,
		Annotations: om.Annotations,
	}
}
