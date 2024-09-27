package objectdeployments

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type objectDeploymentAccessor interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetSelector() metav1.LabelSelector
	GetObjectSetTemplate() corev1alpha1.ObjectSetTemplate
	GetRevisionHistoryLimit() *int32
	SetStatusConditions(...metav1.Condition)
	SetStatusCollisionCount(*int32)
	GetStatusCollisionCount() *int32
	GetStatusTemplateHash() string
	GetGeneration() int64
	SetStatusTemplateHash(templateHash string)
	SetStatusRevision(r int64)
	SetStatusControllerOf([]corev1alpha1.ControlledObjectReference)
	GetStatusControllerOf() []corev1alpha1.ControlledObjectReference
}
