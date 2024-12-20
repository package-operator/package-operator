package packages

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
)

type objectDeploymentStatusReconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	newObjectDeployment adapters.ObjectDeploymentFactory
}

func (r *objectDeploymentStatusReconciler) Reconcile(
	ctx context.Context, packageObj adapters.GenericPackageAccessor,
) (ctrl.Result, error) {
	objDep := r.newObjectDeployment(r.scheme)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(packageObj.ClientObject()), objDep.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	objDepAvailableCond := meta.FindStatusCondition(*objDep.GetConditions(), corev1alpha1.ObjectDeploymentAvailable)
	if objDepAvailableCond != nil && objDepAvailableCond.ObservedGeneration == objDep.ClientObject().GetGeneration() {
		packageAvailableCond := objDepAvailableCond.DeepCopy()
		packageAvailableCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageAvailableCond)
	}

	objDepProgressingCond := meta.FindStatusCondition(*objDep.GetConditions(), corev1alpha1.ObjectDeploymentProgressing)
	if objDepProgressingCond != nil && objDepProgressingCond.ObservedGeneration == objDep.ClientObject().GetGeneration() {
		packageProgressingCond := objDepProgressingCond.DeepCopy()
		packageProgressingCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageProgressingCond)
	}

	objDepPausedCond := meta.FindStatusCondition(*objDep.GetConditions(), corev1alpha1.ObjectDeploymentPaused)
	if objDepPausedCond != nil && objDepPausedCond.ObservedGeneration == objDep.ClientObject().GetGeneration() &&
		objDepPausedCond.Status == metav1.ConditionTrue {
		packagePausedCond := objDepPausedCond.DeepCopy()
		packagePausedCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()
		meta.SetStatusCondition(packageObj.GetConditions(), *packagePausedCond)
	} else {
		meta.RemoveStatusCondition(packageObj.GetConditions(), corev1alpha1.PackagePaused)
	}

	controllers.DeleteMappedConditions(ctx, packageObj.GetConditions())
	controllers.MapConditions(
		ctx,
		objDep.ClientObject().GetGeneration(), *objDep.GetConditions(),
		packageObj.ClientObject().GetGeneration(), packageObj.GetConditions(),
	)

	packageObj.SetStatusRevision(objDep.GetStatusRevision())

	return ctrl.Result{}, nil
}
