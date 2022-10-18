package objectdeployments

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type newRevisionReconciler struct {
	client       client.Client
	newObjectSet genericObjectSetFactory
	scheme       *runtime.Scheme
}

func (r *newRevisionReconciler) Reconcile(ctx context.Context,
	currentObject genericObjectSet,
	prevObjectSets []genericObjectSet,
	objectDeployment genericObjectDeployment) (ctrl.Result, error) {

	if currentObject != nil {
		// There is an objectset already for the current revision, we do nothing.
		return ctrl.Result{}, nil
	}
	log := logr.FromContextOrDiscard(ctx)

	newObjectSet, err := r.newObjectSetFromDeployment(objectDeployment, prevObjectSets)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("errored while trying to create a new objectset in memory: %w", err)
	}

	err = r.client.Create(ctx, newObjectSet.ClientObject())
	if err == nil {
		return ctrl.Result{}, nil
	}

	if err != nil && !errors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("errored while creating new ObjectSet: %w", err)
	}
	// Hash collision
	log.Info("Got hash collision")
	conflictingObjectSet := r.newObjectSet(r.scheme)
	if err := r.client.Get(
		ctx, client.ObjectKeyFromObject(newObjectSet.ClientObject()),
		conflictingObjectSet.ClientObject()); err != nil {
		return ctrl.Result{}, fmt.Errorf("errored when getting conflicting ObjectSet: %w", err)
	}

	// sanity check, before we increment the collision counter
	conflictAnnotations := conflictingObjectSet.ClientObject().GetAnnotations()
	currentDeploymentGeneration := fmt.Sprint(objectDeployment.ClientObject().GetGeneration())
	if conflictAnnotations != nil && conflictAnnotations[DeploymentRevisionAnnotation] == currentDeploymentGeneration {
		// Objectset for the current deployment revision already present, do nothing!
		return ctrl.Result{}, nil
	}

	// Hash collision, we update the collision counter of the objectdeployment
	currentCollisionCount := objectDeployment.GetStatusCollisionCount()
	if currentCollisionCount == nil {
		currentCollisionCount = new(int32)
	}
	*currentCollisionCount++
	objectDeployment.SetStatusCollisionCount(
		currentCollisionCount,
	)

	return ctrl.Result{}, nil
}

// Creates and returns a new objectset in memory with the correct objectset template,
// template hash, previous revision references and ownership set.
func (r *newRevisionReconciler) newObjectSetFromDeployment(
	objectDeployment genericObjectDeployment,
	prevObjectSets []genericObjectSet,
) (genericObjectSet, error) {
	deploymentClientObj := objectDeployment.ClientObject()
	newObjectSet := r.newObjectSet(r.scheme)
	newObjectSetClientObj := newObjectSet.ClientObject()
	newObjectSetClientObj.SetName(deploymentClientObj.GetName() + "-" + objectDeployment.GetStatusTemplateHash())
	newObjectSetClientObj.SetNamespace(deploymentClientObj.GetNamespace())
	newObjectSetClientObj.SetAnnotations(deploymentClientObj.GetAnnotations())
	newObjectSetClientObj.SetLabels(objectDeployment.GetObjectSetTemplate().Metadata.Labels)
	newObjectSet.SetTemplateSpec(
		objectDeployment.GetObjectSetTemplate().Spec,
	)
	newObjectSet.SetPreviousRevisions(prevObjectSets)

	if newObjectSetClientObj.GetAnnotations() == nil {
		newObjectSetClientObj.SetAnnotations(map[string]string{})
	}
	newObjectSetClientObj.GetAnnotations()[ObjectSetHashAnnotation] = objectDeployment.GetStatusTemplateHash()
	newObjectSetClientObj.GetAnnotations()[DeploymentRevisionAnnotation] = fmt.Sprint(deploymentClientObj.GetGeneration())
	if err := controllerutil.SetControllerReference(
		deploymentClientObj, newObjectSetClientObj, r.scheme); err != nil {
		return nil, err
	}
	return newObjectSet, nil
}
