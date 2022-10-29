package packages

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type genericPackage interface {
	ClientObject() client.Object
	Scope() manifestsv1alpha1.PackageManifestScope
	TemplateContext() FolderLoaderTemplateContext
	GetConditions() *[]metav1.Condition
}

type genericPackageFactory func(
	scheme *runtime.Scheme) genericPackage

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

func (a *GenericPackage) Scope() manifestsv1alpha1.PackageManifestScope {
	return manifestsv1alpha1.PackageManifestScopeNamespaced
}

func (a *GenericPackage) TemplateContext() FolderLoaderTemplateContext {
	return FolderLoaderTemplateContext{
		Package: PackageTemplateContext{
			ObjectMeta: a.ObjectMeta,
		},
	}
}

func (a *GenericPackage) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

type GenericClusterPackage struct {
	corev1alpha1.ClusterPackage
}

func (a *GenericClusterPackage) ClientObject() client.Object {
	return &a.ClusterPackage
}

func (a *GenericClusterPackage) Scope() manifestsv1alpha1.PackageManifestScope {
	return manifestsv1alpha1.PackageManifestScopeCluster
}

func (a *GenericClusterPackage) TemplateContext() FolderLoaderTemplateContext {
	return FolderLoaderTemplateContext{
		Package: PackageTemplateContext{
			ObjectMeta: a.ObjectMeta,
		},
	}
}

func (a *GenericClusterPackage) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}
