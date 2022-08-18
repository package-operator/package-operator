package webhooks

import (
	"k8s.io/apimachinery/pkg/api/equality"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func validateObjectSetPhaseImmutability(osp, oldOsp *corev1alpha1.ObjectSetPhase) error {
	if !equality.Semantic.DeepEqual(osp.Spec.ObjectSetTemplatePhase, oldOsp.Spec.ObjectSetTemplatePhase) {
		return errObjectSetTemplatePhaseImmutable
	}

	if !equality.Semantic.DeepEqual(osp.Spec.Previous, oldOsp.Spec.Previous) {
		return errPreviousImmutable
	}

	if osp.Spec.Revision != oldOsp.Spec.Revision {
		return errRevisionImmutable
	}

	if !equality.Semantic.DeepEqual(osp.Spec.AvailabilityProbes, oldOsp.Spec.AvailabilityProbes) {
		return errAvailabilityProbesImmutable
	}

	return nil
}
