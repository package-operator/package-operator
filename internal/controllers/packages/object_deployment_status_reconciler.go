package packages

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

type objectDeploymentStatusReconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	newObjectDeployment genericObjectDeploymentFactory
}

func newObjectDeploymentStatusReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	newObjectDeployment genericObjectDeploymentFactory,
) *objectDeploymentStatusReconciler {
	return &objectDeploymentStatusReconciler{
		client:              c,
		scheme:              scheme,
		newObjectDeployment: newObjectDeployment,
	}
}

func (r *objectDeploymentStatusReconciler) Reconcile(ctx context.Context, packageObj genericPackage) (ctrl.Result, error) {
	objDep := r.newObjectDeployment(r.scheme)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(packageObj.ClientObject()), objDep.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	objDepAvailableCondition := meta.FindStatusCondition(objDep.GetConditions(), corev1alpha1.ObjectDeploymentAvailable)
	if objDepAvailableCondition != nil && objDepAvailableCondition.ObservedGeneration == objDep.ClientObject().GetGeneration() {
		packageAvailableCond := objDepAvailableCondition.DeepCopy()
		packageAvailableCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageAvailableCond)
	}

	objDepProgressingCondition := meta.FindStatusCondition(objDep.GetConditions(), corev1alpha1.ObjectDeploymentProgressing)
	if objDepProgressingCondition != nil && objDepProgressingCondition.ObservedGeneration == objDep.ClientObject().GetGeneration() {
		packageProgressingCond := objDepProgressingCondition.DeepCopy()
		packageProgressingCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageProgressingCond)
	}

	return ctrl.Result{}, nil
}
