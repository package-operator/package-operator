package packages

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers"
)

func (c *GenericPackageController[P, D]) reconcileObjectDeploymentStatus(ctx context.Context, pkg *P) (ctrl.Result, error) {
	var d D

	depl := &d
	deplObj := any(depl).(client.Object)
	deplConditions := ConditionsPtr(depl)

	pkgObj := any(pkg).(client.Object)
	pkgConditions := ConditionsPtr(pkg)

	if err := c.client.Get(ctx, client.ObjectKeyFromObject(pkgObj), deplObj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	objDepAvailableCondition := meta.FindStatusCondition(*deplConditions, corev1alpha1.ObjectDeploymentAvailable)
	if objDepAvailableCondition != nil && objDepAvailableCondition.ObservedGeneration == deplObj.GetGeneration() {
		packageAvailableCond := objDepAvailableCondition.DeepCopy()
		packageAvailableCond.ObservedGeneration = deplObj.GetGeneration()

		meta.SetStatusCondition(pkgConditions, *packageAvailableCond)
	}

	objDepProgressingCondition := meta.FindStatusCondition(*deplConditions, corev1alpha1.ObjectDeploymentProgressing)
	if objDepProgressingCondition != nil && objDepProgressingCondition.ObservedGeneration == deplObj.GetGeneration() {
		packageProgressingCond := objDepProgressingCondition.DeepCopy()
		packageProgressingCond.ObservedGeneration = pkgObj.GetGeneration()

		meta.SetStatusCondition(pkgConditions, *packageProgressingCond)
	}

	controllers.DeleteMappedConditions(ctx, pkgConditions)
	controllers.MapConditions(
		ctx,
		deplObj.GetGeneration(), *deplConditions,
		pkgObj.GetGeneration(), pkgConditions,
	)

	*StatusRevisionPtr(pkg) = *StatusRevisionPtr(depl)

	return ctrl.Result{}, nil
}
