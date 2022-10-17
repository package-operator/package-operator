package objectdeployments

import (
	"context"

	"package-operator.run/package-operator/internal/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type hashReconciler struct {
	client client.Client
}

func (h *hashReconciler) Reconcile(ctx context.Context, objectSetDeployment genericObjectDeployment) (ctrl.Result, error) {
	objectSetTemplate := objectSetDeployment.GetObjectSetTemplate()
	templateHash := utils.ComputeHash(objectSetTemplate, objectSetDeployment.GetStatusCollisionCount())
	objectSetDeployment.SetStatusTemplateHash(templateHash)
	return ctrl.Result{}, nil
}
