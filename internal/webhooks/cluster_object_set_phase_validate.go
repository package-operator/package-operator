package webhooks

import (
	"k8s.io/apimachinery/pkg/api/equality"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func validateClusterObjectSetPhaseImmutability(cosp, oldCosp *corev1alpha1.ClusterObjectSetPhase) error {
	if !equality.Semantic.DeepEqual(cosp.Spec.ObjectSetTemplatePhase, oldCosp.Spec.ObjectSetTemplatePhase) {
		return errObjectSetTemplatePhaseImmutable
	}

	if !equality.Semantic.DeepEqual(cosp.Spec.Previous, oldCosp.Spec.Previous) {
		return errPreviousImmutable
	}

	if cosp.Spec.Revision != oldCosp.Spec.Revision {
		return errRevisionImmutable
	}

	if !equality.Semantic.DeepEqual(cosp.Spec.AvailabilityProbes, oldCosp.Spec.AvailabilityProbes) {
		return errAvailabilityProbesImmutable
	}

	return nil
}
