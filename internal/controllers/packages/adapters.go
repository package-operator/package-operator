package packages

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type objectDeploymentAccessor interface {
	ClientObject() client.Object
	GetPhases() []corev1alpha1.ObjectSetTemplatePhase
	SetPhases(phases []corev1alpha1.ObjectSetTemplatePhase)
	GetConditions() []metav1.Condition
	GetObjectMeta() metav1.ObjectMeta
	SetObjectMeta(metav1.ObjectMeta)
}
