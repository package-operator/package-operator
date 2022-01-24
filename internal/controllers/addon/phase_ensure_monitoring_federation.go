package addon

import (
	"context"
	"errors"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
)

// Ensure existence of ServiceMonitors for monitoring configuration specified in the given Addon resource.
func (r *AddonReconciler) ensureMonitoringFederation(
	ctx context.Context,
	addon *addonsv1alpha1.Addon,
) (stop bool, err error) {
	// early return if .spec.monitoring.federation is not specified
	if addon.Spec.Monitoring == nil || addon.Spec.Monitoring.Federation == nil {
		return false, nil
	}

	// ensure monitoring namespace
	monitoringNamespaceName := controllers.GetMonitoringNamespaceName(addon)
	if monitoringNamespace, err := r.ensureNamespaceWithLabels(ctx, addon, monitoringNamespaceName, map[string]string{
		"openshift.io/cluster-monitoring": "true",
	}); err != nil {
		if errors.Is(err, controllers.ErrNotOwnedByUs) {
			return true, nil
		}
		return false, fmt.Errorf("could not ensure monitoring Namespace: %w", err)
	} else if monitoringNamespace.Status.Phase != corev1.NamespaceActive {
		meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
			Type:   addonsv1alpha1.Available,
			Status: metav1.ConditionFalse,
			Reason: addonsv1alpha1.AddonReasonUnreadyMonitoring,
			Message: fmt.Sprintf(
				"Monitoring Namespace is not in Active phase: %s",
				monitoringNamespaceName,
			),
			ObservedGeneration: addon.Generation,
		})
		addon.Status.ObservedGeneration = addon.Generation
		addon.Status.Phase = addonsv1alpha1.PhasePending
		return false, r.Status().Update(ctx, addon)
	}

	desiredServiceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllers.GetMonitoringFederationServiceMonitorName(addon),
			Namespace: monitoringNamespaceName,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					HonorLabels: true,
					Port:        "9090",
					Path:        "/federate",
					Scheme:      "https",
					Interval:    "30s",
					TLSConfig: &monitoringv1.TLSConfig{
						CAFile: "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt",
						SafeTLSConfig: monitoringv1.SafeTLSConfig{
							ServerName: fmt.Sprintf(
								"prometheus.%s.svc",
								addon.Spec.Monitoring.Federation.Namespace,
							),
						},
					},
				},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{addon.Spec.Monitoring.Federation.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: addon.Spec.Monitoring.Federation.MatchLabels,
			},
		},
	}

	matchParams := []string{
		`ALERTS{alertstate="firing"}`,
	}
	for _, matchName := range addon.Spec.Monitoring.Federation.MatchNames {
		matchParams = append(matchParams, fmt.Sprintf(`{__name__="%s"}`, matchName))
	}
	desiredServiceMonitor.Spec.Endpoints[0].Params = map[string][]string{
		"match[]": matchParams,
	}

	controllers.AddCommonLabels(desiredServiceMonitor.Labels, addon)

	if err := controllerutil.SetControllerReference(addon, desiredServiceMonitor, r.Scheme); err != nil {
		return false, fmt.Errorf("could not set controller reference on ServiceMonitor: %w", err)
	}

	if err := r.reconcileServiceMonitor(ctx, desiredServiceMonitor, addon.Spec.ResourceAdoptionStrategy); err != nil {
		return false, fmt.Errorf("could not reconcile ServiceMonitor: %w", err)
	}

	return false, nil
}

// Reconciles the Spec of the given ServiceMonitor if needed by updating or creating the ServiceMonitor.
// If a change happens, the given ServiceMonitor is updated to reflect the latest state from the kube-apiserver.
func (r *AddonReconciler) reconcileServiceMonitor(
	ctx context.Context,
	serviceMonitor *monitoringv1.ServiceMonitor,
	strategy addonsv1alpha1.ResourceAdoptionStrategyType,
) error {
	currentServiceMonitor := &monitoringv1.ServiceMonitor{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(serviceMonitor), currentServiceMonitor); k8sApiErrors.IsNotFound(err) {
		return r.Create(ctx, serviceMonitor)
	} else if err != nil {
		return fmt.Errorf("getting ServiceMonitor: %w", err)
	}

	if len(currentServiceMonitor.OwnerReferences) == 0 ||
		!controllers.HasEqualControllerReference(currentServiceMonitor, serviceMonitor) {
		// TODO: remove this condition once resourceAdoptionStrategy is discontinued
		// Only enforce resource-adoption check for resources NOT owned by the Addon in the first place.
		// Note: `serviceMonitor`'s ownerRef is the Addon.
		if strategy != addonsv1alpha1.ResourceAdoptionAdoptAll && !controllers.HasEqualControllerReference(currentServiceMonitor, serviceMonitor) {
			return controllers.ErrNotOwnedByUs
		}
	}

	if !equality.Semantic.DeepEqual(currentServiceMonitor.Spec, serviceMonitor.Spec) {
		currentServiceMonitor.Spec = serviceMonitor.Spec
		return r.Update(ctx, currentServiceMonitor)
	}

	return nil
}
