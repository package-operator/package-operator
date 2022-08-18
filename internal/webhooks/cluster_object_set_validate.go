package webhooks

import (
	"k8s.io/apimachinery/pkg/api/equality"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func validateClusterObjectSetImmutability(cos, oldCos *corev1alpha1.ClusterObjectSet) error {
	if !equality.Semantic.DeepEqual(cos.Spec.ObjectSetTemplateSpec, oldCos.Spec.ObjectSetTemplateSpec) {
		return errObjectSetTemplateSpecImmutable
	}

	if !equality.Semantic.DeepEqual(cos.Spec.Previous, oldCos.Spec.Previous) {
		return errPreviousImmutable
	}

	//// Do semantic DeepEqual instead of reflect.DeepEqual  TODO: Why??
	//if !equality.Semantic.DeepEqual(oldSpecInstall, specInstall) {

	return nil
}
