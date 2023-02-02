package packagedeploy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type (
	GenericClusterPackage struct{ corev1alpha1.ClusterPackage }
	GenericPackage        struct{ corev1alpha1.Package }
)

type (
	genericPackage interface {
		ClientObject() client.Object
		TemplateContext() manifestsv1alpha1.TemplateContext
		GetConditions() *[]metav1.Condition
	}

	genericPackageFactory func(scheme *runtime.Scheme) genericPackage
)

var (
	packageGVK                       = corev1alpha1.GroupVersion.WithKind("Package")
	clusterPackageGVK                = corev1alpha1.GroupVersion.WithKind("ClusterPackage")
	_                 genericPackage = (*GenericPackage)(nil)
	_                 genericPackage = (*GenericClusterPackage)(nil)
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

	return &GenericClusterPackage{ClusterPackage: *obj.(*corev1alpha1.ClusterPackage)}
}

func (a *GenericPackage) TemplateContext() manifestsv1alpha1.TemplateContext {
	return manifestsv1alpha1.TemplateContext{
		Package: manifestsv1alpha1.TemplateContextPackage{
			TemplateContextObjectMeta: templateContextObjectMetaFromObjectMeta(a.ObjectMeta),
		},
		Config: a.Package.Spec.Config,
	}
}

func (a *GenericPackage) ClientObject() client.Object        { return &a.Package }
func (a *GenericPackage) GetConditions() *[]metav1.Condition { return &a.Status.Conditions }

func (a *GenericClusterPackage) ClientObject() client.Object        { return &a.ClusterPackage }
func (a *GenericClusterPackage) GetConditions() *[]metav1.Condition { return &a.Status.Conditions }

func (a *GenericClusterPackage) TemplateContext() manifestsv1alpha1.TemplateContext {
	return manifestsv1alpha1.TemplateContext{
		Package: manifestsv1alpha1.TemplateContextPackage{
			TemplateContextObjectMeta: templateContextObjectMetaFromObjectMeta(a.ObjectMeta),
		},
		Config: a.Spec.Config,
	}
}

func templateContextObjectMetaFromObjectMeta(om metav1.ObjectMeta) manifestsv1alpha1.TemplateContextObjectMeta {
	return manifestsv1alpha1.TemplateContextObjectMeta{
		Name:        om.Name,
		Namespace:   om.Namespace,
		Labels:      om.Labels,
		Annotations: om.Annotations,
	}
}
