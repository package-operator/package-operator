package controllers

import (
	"context"
	"fmt"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/addon-operator/apis"
	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func (r *AddonReconciler) observeCurrentCSV(
	ctx context.Context,
	addon *addonsv1alpha1.Addon,
	csvKey client.ObjectKey,
) (requeue bool, err error) {
	csv := &operatorsv1alpha1.ClusterServiceVersion{}
	if err := r.Get(ctx, csvKey, csv); err != nil {
		return false, fmt.Errorf("getting installed CSV: %w", err)
	}

	var message string
	switch csv.Status.Phase {
	case operatorsv1alpha1.CSVPhaseSucceeded:
		// do nothing here
	case operatorsv1alpha1.CSVPhaseFailed:
		message = "failed"
	default:
		message = "unkown/pending"
	}

	if message != "" {
		meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
			Type:   addonsv1alpha1.Available,
			Status: metav1.ConditionFalse,
			Reason: apis.AddonReasonUnreadyCSV,
			Message: fmt.Sprintf(
				"ClusterServiceVersion is not ready: %s",
				message),
			ObservedGeneration: addon.Generation,
		})
		addon.Status.ObservedGeneration = addon.Generation
		addon.Status.Phase = addonsv1alpha1.PhasePending
		return true, r.Status().Update(ctx, addon)
	}

	return false, nil
}
