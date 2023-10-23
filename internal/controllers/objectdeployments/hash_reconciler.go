package objectdeployments

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"package-operator.run/internal/utils"
)

type hashReconciler struct{ client client.Client }

func (h *hashReconciler) Reconcile(
	_ context.Context, objectSetDeployment objectDeploymentAccessor,
) (ctrl.Result, error) {
	objectSetTemplate := objectSetDeployment.GetObjectSetTemplate()
	templateHash := utils.ComputeFNV32Hash(objectSetTemplate, objectSetDeployment.GetStatusCollisionCount())
	objectSetDeployment.SetStatusTemplateHash(templateHash)
	return ctrl.Result{}, nil
}
