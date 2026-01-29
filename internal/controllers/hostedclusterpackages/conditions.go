package hostedclusterpackages

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func packageConditionIs(pkg *corev1alpha1.Package, conditionType string, status metav1.ConditionStatus) bool {
	cond := meta.FindStatusCondition(pkg.Status.Conditions, conditionType)
	return cond != nil && cond.Status == status && pkg.GetGeneration() == cond.ObservedGeneration
}

func isPackageAvailable(pkg *corev1alpha1.Package) bool {
	return packageConditionIs(pkg, corev1alpha1.PackageAvailable, metav1.ConditionTrue)
}

func isPackageProgressed(pkg *corev1alpha1.Package) bool {
	return packageConditionIs(pkg, corev1alpha1.PackageProgressing, metav1.ConditionFalse) &&
		packageConditionIs(pkg, corev1alpha1.PackageUnpacked, metav1.ConditionTrue)
}

func isPackagePaused(pkg *corev1alpha1.Package) bool {
	return packageConditionIs(pkg, corev1alpha1.PackagePaused, metav1.ConditionTrue)
}
