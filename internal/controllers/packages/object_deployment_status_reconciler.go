package packages

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
)

type objectDeploymentStatusReconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	newObjectDeployment adapters.ObjectDeploymentFactory
}

func (r *objectDeploymentStatusReconciler) Reconcile(ctx context.Context, packageObj genericPackage) (ctrl.Result, error) {
	objDep := r.newObjectDeployment(r.scheme)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(packageObj.ClientObject()), objDep.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	objDepAvailableCondition := meta.FindStatusCondition(*objDep.GetConditions(), corev1alpha1.ObjectDeploymentAvailable)
	if objDepAvailableCondition != nil && objDepAvailableCondition.ObservedGeneration == objDep.ClientObject().GetGeneration() {
		packageAvailableCond := objDepAvailableCondition.DeepCopy()
		packageAvailableCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageAvailableCond)
	}

	objDepProgressingCondition := meta.FindStatusCondition(*objDep.GetConditions(), corev1alpha1.ObjectDeploymentProgressing)
	if objDepProgressingCondition != nil && objDepProgressingCondition.ObservedGeneration == objDep.ClientObject().GetGeneration() {
		packageProgressingCond := objDepProgressingCondition.DeepCopy()
		packageProgressingCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageProgressingCond)
	}

	// map conditions
	// -> copy mapped status conditions
	for _, condition := range *objDep.GetConditions() {
		if condition.ObservedGeneration !=
			objDep.ClientObject().GetGeneration() {
			// mapped condition is outdated
			continue
		}

		if !strings.Contains(condition.Type, "/") {
			// mapped conditions are prefixed
			continue
		}

		meta.SetStatusCondition(packageObj.GetConditions(), metav1.Condition{
			Type:               condition.Type,
			Status:             condition.Status,
			Reason:             condition.Reason,
			Message:            condition.Message,
			ObservedGeneration: packageObj.ClientObject().GetGeneration(),
		})
	}

	return ctrl.Result{}, nil
}
