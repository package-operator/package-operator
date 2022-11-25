package objectdeployments

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	objectDeployment objectDeploymentAccessor) (ctrl.Result, error) {

	if currentObject != nil {
		// There is an objectset already for the current revision, we do nothing.
		return ctrl.Result{}, nil
	}
	log := logr.FromContextOrDiscard(ctx)

	if len(objectDeployment.GetObjectSetTemplate().Spec.Phases) == 0 {
		// ObjectDeployment is empty. Don't create a ObjectSet, wait for spec.
		log.Info("empty ObjectDeployment, waiting for initialization")
		return ctrl.Result{}, nil
	}

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

	conflictingObjectSet := r.newObjectSet(r.scheme)
	if err := r.client.Get(
		ctx, client.ObjectKeyFromObject(newObjectSet.ClientObject()), conflictingObjectSet.ClientObject(),
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting conflicting ObjectSet: %w", err)
	}
	controllerRef := metav1.GetControllerOf(conflictingObjectSet.ClientObject())
	if controllerRef != nil &&
		controllerRef.UID == objectDeployment.ClientObject().GetUID() &&
		equality.Semantic.DeepEqual(newObjectSet.GetTemplateSpec(), conflictingObjectSet.GetTemplateSpec()) {
		// This ObjectDeployment is controller of the conflicting ObjectSet and the ObjectSet is deep equal to the desired new ObjectSet.
		// So no conflict :) This case can happen if the local cache is a little bit slow to record the ObjectSet Create event.
		log.Info("Slow cache, no collision")
		return ctrl.Result{}, nil
	}

	log.Info("Got hash collision")
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
	objectDeployment objectDeploymentAccessor,
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
	if err := controllerutil.SetControllerReference(
		deploymentClientObj, newObjectSetClientObj, r.scheme); err != nil {
		return nil, err
	}
	return newObjectSet, nil
}
