package addon

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// use this type for exit handling
type requeueResult int

const (
	// Should be used when requeue result does not matter.
	// For example, when an error is returned along with it.
	resultNil requeueResult = iota

	// Should be used when request needs to be retried
	resultRetry

	// Should be used when reconciler needs to stop and exit.
	resultStop
)

// This method should be called ONLY if result is NOT `resultNil`, or it could
// lead to unpredictable behaviour.
func (r *AddonReconciler) handleExit(result requeueResult) ctrl.Result {
	switch result {
	case resultRetry:
		return ctrl.Result{
			RequeueAfter: defaultRetryAfterTime,
		}
	default:
		return ctrl.Result{}
	}
}

// Handle the deletion of an Addon.
func (r *AddonReconciler) handleAddonDeletion(
	ctx context.Context, addon *addonsv1alpha1.Addon,
) error {
	if !controllerutil.ContainsFinalizer(addon, cacheFinalizer) {
		// The finalizer is already gone and the deletion timestamp is set.
		// kube-apiserver should have garbage collected this object already,
		// this delete signal does not need further processing.
		return nil
	}

	reportTerminationStatus(addon)

	// Clear from CSV Event Handler
	r.csvEventHandler.Free(addon)

	controllerutil.RemoveFinalizer(addon, cacheFinalizer)
	if err := r.Update(ctx, addon); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}

// Report Addon status to communicate that everything is alright
func reportReadinessStatus(addon *addonsv1alpha1.Addon) {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Available,
		Status:             metav1.ConditionTrue,
		Reason:             addonsv1alpha1.AddonReasonFullyReconciled,
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseReady

}

// Report Addon status to communicate that the Addon is terminating
func reportTerminationStatus(addon *addonsv1alpha1.Addon) {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Available,
		Status:             metav1.ConditionFalse,
		Reason:             addonsv1alpha1.AddonReasonTerminating,
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseTerminating
}

// Report Addon status to communicate that the resource is misconfigured
func reportConfigurationError(addon *addonsv1alpha1.Addon, message string) {
	// TODO: remove the following 2 lines of code
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseError
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:    addonsv1alpha1.Available,
		Status:  metav1.ConditionFalse,
		Reason:  addonsv1alpha1.AddonReasonConfigError,
		Message: message,
	})
	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhaseError
}

// Marks Addon as paused
func reportAddonPauseStatus(addon *addonsv1alpha1.Addon,
	reason string) {
	meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
		Type:               addonsv1alpha1.Paused,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            "",
		ObservedGeneration: addon.Generation,
	})
	addon.Status.ObservedGeneration = addon.Generation
}

// remove Paused condition from Addon
func (r *AddonReconciler) removeAddonPauseCondition(addon *addonsv1alpha1.Addon) {
	meta.RemoveStatusCondition(&addon.Status.Conditions, addonsv1alpha1.Paused)
	addon.Status.ObservedGeneration = addon.Generation
}

// Marks Addon as unavailable because the CatalogSource is unready
func reportCatalogSourceUnreadinessStatus(addon *addonsv1alpha1.Addon, message string) {
	reportPendingStatus(addon, addonsv1alpha1.AddonReasonUnreadyCatalogSource,
		fmt.Sprintf("CatalogSource connection is not ready: %s", message))
}

func reportUnreadyNamespaces(addon *addonsv1alpha1.Addon, unreadyNamespaces []string) {
	reportPendingStatus(addon, addonsv1alpha1.AddonReasonUnreadyNamespaces,
		fmt.Sprintf("Namespaces not yet in Active phase: %s", strings.Join(unreadyNamespaces, ", ")))
}

func reportCollidedNamespaces(addon *addonsv1alpha1.Addon, collidedNamespaces []string) {
	reportPendingStatus(addon, addonsv1alpha1.AddonReasonCollidedNamespaces,
		fmt.Sprintf("Namespaces with collisions: %s", strings.Join(collidedNamespaces, ", ")))
}

func reportUnreadyCSV(addon *addonsv1alpha1.Addon, message string) {
	reportPendingStatus(addon, addonsv1alpha1.AddonReasonUnreadyCSV,
		fmt.Sprintf("ClusterServiceVersion is not ready: %s", message))
}

func reportUnreadyMonitoring(addon *addonsv1alpha1.Addon, message string) {
	reportPendingStatus(addon, addonsv1alpha1.AddonReasonUnreadyMonitoring,
		fmt.Sprintf("Monitoring Federation is not ready: %s", message))
}

func reportPendingStatus(addon *addonsv1alpha1.Addon, reason, msg string) {
	meta.SetStatusCondition(&addon.Status.Conditions,
		metav1.Condition{
			Type:               addonsv1alpha1.Available,
			Status:             metav1.ConditionFalse,
			Reason:             reason,
			Message:            msg,
			ObservedGeneration: addon.Generation,
		})

	addon.Status.ObservedGeneration = addon.Generation
	addon.Status.Phase = addonsv1alpha1.PhasePending
}

// Validate addon.Spec.Install then extract
// targetNamespace and catalogSourceImage from it
func (r *AddonReconciler) parseAddonInstallConfig(
	log logr.Logger, addon *addonsv1alpha1.Addon) (
	targetNamespace, catalogSourceImage string, stop bool,
) {
	switch addon.Spec.Install.Type {
	case addonsv1alpha1.OLMOwnNamespace:
		if addon.Spec.Install.OLMOwnNamespace == nil ||
			len(addon.Spec.Install.OLMOwnNamespace.Namespace) == 0 {
			// invalid/missing configuration
			reportConfigurationError(addon,
				".spec.install.ownNamespace.namespace is required when .spec.install.type = OwnNamespace")
			return "", "", true
		}
		targetNamespace = addon.Spec.Install.OLMOwnNamespace.Namespace
		if len(addon.Spec.Install.OLMOwnNamespace.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			reportConfigurationError(addon,
				".spec.install.ownNamespacee.catalogSourceImage is"+
					"required when .spec.install.type = OwnNamespace")
			return "", "", true
		}
		catalogSourceImage = addon.Spec.Install.OLMOwnNamespace.CatalogSourceImage

	case addonsv1alpha1.OLMAllNamespaces:
		if addon.Spec.Install.OLMAllNamespaces == nil ||
			len(addon.Spec.Install.OLMAllNamespaces.Namespace) == 0 {
			// invalid/missing configuration
			reportConfigurationError(addon,
				".spec.install.allNamespaces.namespace is required when"+
					" .spec.install.type = AllNamespaces")
			return "", "", true
		}
		targetNamespace = addon.Spec.Install.OLMAllNamespaces.Namespace
		if len(addon.Spec.Install.OLMAllNamespaces.CatalogSourceImage) == 0 {
			// invalid/missing configuration
			reportConfigurationError(addon,
				".spec.install.allNamespaces.catalogSourceImage is required"+
					"when .spec.install.type = AllNamespaces")
			return "", "", true
		}
		catalogSourceImage = addon.Spec.Install.OLMAllNamespaces.CatalogSourceImage

	default:
		// Unsupported Install Type
		// This should never happen, unless the schema validation is wrong.
		// The .install.type property is set to only allow known enum values.
		log.Error(fmt.Errorf("invalid Addon install type: %q", addon.Spec.Install.Type),
			"stopping Addon reconcilation")
		return "", "", true
	}
	return targetNamespace, catalogSourceImage, false
}

// HasMonitoringFederation is a helper to determine if a given addon's spec
// defines a Monitoring.Federation.
func HasMonitoringFederation(addon *addonsv1alpha1.Addon) bool {
	return addon.Spec.Monitoring != nil && addon.Spec.Monitoring.Federation != nil
}

// Helper function to compute monitoring Namespace name from addon object
func GetMonitoringNamespaceName(addon *addonsv1alpha1.Addon) string {
	return fmt.Sprintf("redhat-monitoring-%s", addon.Name)
}

// Helper function to compute monitoring federation ServiceMonitor name from addon object
func GetMonitoringFederationServiceMonitorName(addon *addonsv1alpha1.Addon) string {
	return fmt.Sprintf("federated-sm-%s", addon.Name)
}

// GetMonitoringFederationServiceMonitorEndpoints generates a slice of monitoringv1.Endpoint
// instances from an addon's Monitoring.Federation specification.
func GetMonitoringFederationServiceMonitorEndpoints(addon *addonsv1alpha1.Addon) []monitoringv1.Endpoint {
	const cacert = "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt"

	tlsConfig := &monitoringv1.TLSConfig{
		CAFile: cacert,
		SafeTLSConfig: monitoringv1.SafeTLSConfig{
			ServerName: fmt.Sprintf("prometheus.%s.svc", addon.Spec.Monitoring.Federation.Namespace),
		},
	}

	matchParams := []string{`ALERTS{alertstate="firing"}`}

	for _, name := range addon.Spec.Monitoring.Federation.MatchNames {
		matchParams = append(matchParams, fmt.Sprintf(`{__name__="%s"}`, name))
	}

	return []monitoringv1.Endpoint{{
		HonorLabels: true,
		Port:        "9090",
		Path:        "/federate",
		Scheme:      "https",
		Interval:    "30s",
		TLSConfig:   tlsConfig,
		Params:      map[string][]string{"match[]": matchParams},
	}}
}

// HasAdoptAllStrategy returns true if a given addon has the AdoptAll ResourceAdoptionStrategy.
func HasAdoptAllStrategy(addon *addonsv1alpha1.Addon) bool {
	return addon.Spec.ResourceAdoptionStrategy == addonsv1alpha1.ResourceAdoptionAdoptAll
}
