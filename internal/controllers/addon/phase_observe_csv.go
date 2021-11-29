package addon

import (
	"context"
	"fmt"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func (r *AddonReconciler) observeCurrentCSV(
	ctx context.Context,
	addon *addonsv1alpha1.Addon,
	csvKey client.ObjectKey,
) (requeueResult, error) {
	csv := &operatorsv1alpha1.ClusterServiceVersion{}
	if err := r.Get(ctx, csvKey, csv); err != nil {
		return resultNil, fmt.Errorf("getting installed CSV: %w", err)
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
		reportUnreadyCSV(addon, message)
		return resultRetry, nil
	}

	return resultNil, nil
}
