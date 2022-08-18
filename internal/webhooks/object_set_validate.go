package webhooks

import (
	"k8s.io/apimachinery/pkg/api/equality"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func validateObjectSetImmutability(os, oldOs *corev1alpha1.ObjectSet) error {
	if !equality.Semantic.DeepEqual(os.Spec.ObjectSetTemplateSpec, oldOs.Spec.ObjectSetTemplateSpec) {
		return errObjectSetTemplateSpecImmutable
	}

	if !equality.Semantic.DeepEqual(os.Spec.Previous, oldOs.Spec.Previous) {
		return errPreviousImmutable
	}

	return nil
}
