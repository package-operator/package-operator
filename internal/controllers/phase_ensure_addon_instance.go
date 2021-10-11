package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Ensures the presense or absense of an OperatorGroup depending on the Addon install type.
func (r *AddonReconciler) ensureAddonInstance(
	ctx context.Context, log logr.Logger, addon *addonsv1alpha1.Addon) (err error) {
	// not capturing "stop" because it won't ever be reached due to the guard rails of CRD Enum-Validation Markers
	targetNamespace, _, _, err := r.parseAddonInstallConfig(ctx, log, addon)
	if err != nil {
		return err
	}

	desiredAddonInstance := &addonsv1alpha1.AddonInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addonsv1alpha1.DefaultAddonInstanceName,
			Namespace: targetNamespace,
		},
		Spec: addonsv1alpha1.AddonInstanceSpec{
			HeartbeatUpdatePeriod: int64(10 * time.Second),
		},
	}

	if err := controllerutil.SetControllerReference(addon, desiredAddonInstance, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference: %w", err)
	}

	return r.reconcileAddonInstance(ctx, desiredAddonInstance)
}

// Reconciles the Spec of the given OperatorGroup if needed by updating or creating the OperatorGroup.
// The given OperatorGroup is updated to reflect the latest state from the kube-apiserver.
func (r *AddonReconciler) reconcileAddonInstance(
	ctx context.Context, addonInstance *addonsv1alpha1.AddonInstance) error {
	currentAddonInstance := &addonsv1alpha1.AddonInstance{}

	err := r.Get(ctx, client.ObjectKeyFromObject(addonInstance), currentAddonInstance)
	if errors.IsNotFound(err) {
		return r.Create(ctx, addonInstance)
	}
	if err != nil {
		return fmt.Errorf("getting AddonInstance: %w", err)
	}
	if !equality.Semantic.DeepEqual(currentAddonInstance.Spec, addonInstance.Spec) {
		return r.Update(ctx, addonInstance)
	}
	return nil
}
