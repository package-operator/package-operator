package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Ensures the presence of an AddonInstance well-compliant with the provided Addon object
func (r *AddonReconciler) ensureAddonInstance(
	ctx context.Context, log logr.Logger, addon *addonsv1alpha1.Addon) (err error) {
	// not capturing "stop" because it won't ever be reached due to the guard rails of CRD Enum-Validation Markers
	targetNamespace, _, stop, err := r.parseAddonInstallConfig(ctx, log, addon)
	if err != nil {
		return err
	}
	if stop {
		return fmt.Errorf("failed to create addonInstance due to misconfigured install.spec.type")
	}

	desiredAddonInstance := &addonsv1alpha1.AddonInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      addonsv1alpha1.DefaultAddonInstanceName,
			Namespace: targetNamespace,
		},
		// Can't skip specifying spec because in this case, the zero-value for metav1.Duration will be perceived beforehand i.e. 0s instead of CRD's default value of 10s
		Spec: addonsv1alpha1.AddonInstanceSpec{
			HeartbeatUpdatePeriod: addonsv1alpha1.DefaultAddonInstanceHeartbeatUpdatePeriod,
		},
	}

	if err := controllerutil.SetControllerReference(addon, desiredAddonInstance, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference: %w", err)
	}

	return r.reconcileAddonInstance(ctx, desiredAddonInstance)
}

// Reconciles the reality to have the desired AddonInstance resource by creating it if it does not exist,
// or updating if it exists with a different spec.
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
