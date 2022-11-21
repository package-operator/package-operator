package packages

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type objectDeploymentAccessor interface {
	ClientObject() client.Object
	SetTemplateSpec(corev1alpha1.ObjectSetTemplateSpec)
	GetTemplateSpec() corev1alpha1.ObjectSetTemplateSpec
	SetSelector(labels map[string]string)
	GetSelector() metav1.LabelSelector
}
