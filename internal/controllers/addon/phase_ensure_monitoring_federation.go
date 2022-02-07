package addon

import (
	"context"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
)

// ensureMonitoringFederation inspects an addon's MonitoringFederation specification
// and if it exists ensures that a ServiceMonitor is present in the desired monitoring
// namespace.
func (r *AddonReconciler) ensureMonitoringFederation(ctx context.Context, addon *addonsv1alpha1.Addon) error {
	if !HasMonitoringFederation(addon) {
		return nil
	}

	if err := r.ensureMonitoringNamespace(ctx, addon); err != nil {
		return fmt.Errorf("ensuring monitoring Namespace: %w", err)
	}

	if err := r.ensureServiceMonitor(ctx, addon); err != nil {
		return fmt.Errorf("ensuring ServiceMonitor: %w", err)
	}

	return nil
}

func (r *AddonReconciler) ensureMonitoringNamespace(
	ctx context.Context, addon *addonsv1alpha1.Addon) error {
	desired, err := r.desiredMonitoringNamespace(addon)
	if err != nil {
		return err
	}

	actual, err := r.actualMonitoringNamespace(ctx, addon)
	if k8sApiErrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	} else if err != nil {
		return fmt.Errorf("getting monitoring namespace: %w", err)
	}

	var (
		mustAdopt     = !controllers.HasEqualControllerReference(actual, desired)
		labelsChanged = !equality.Semantic.DeepEqual(actual.Labels, desired.Labels)
	)

	if !mustAdopt && !labelsChanged {
		return nil
	}

	// TODO: remove this condition once resourceAdoptionStrategy is discontinued
	if mustAdopt && !HasAdoptAllStrategy(addon) {
		return controllers.ErrNotOwnedByUs
	}

	actual.OwnerReferences, actual.Labels = desired.OwnerReferences, desired.Labels

	if err := r.Update(ctx, actual); err != nil {
		return fmt.Errorf("updating monitoring namespace: %w", err)
	}

	if actual.Status.Phase == corev1.NamespaceActive {
		return nil
	}

	reportUnreadyMonitoring(addon, fmt.Sprintf("namespace %q is not active", actual.Name))

	// Previously this would trigger exit and move on to the next phase.
	// However given that the reconciliation is not complete an error should
	// be returned to requeue the work.
	return fmt.Errorf("monitoring namespace is not active")
}

func (r *AddonReconciler) desiredMonitoringNamespace(addon *addonsv1alpha1.Addon) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetMonitoringNamespaceName(addon),
			Labels: map[string]string{
				"openshift.io/cluster-monitoring": "true",
			},
		},
	}

	controllers.AddCommonLabels(namespace.Labels, addon)

	if err := controllerutil.SetControllerReference(addon, namespace, r.Scheme); err != nil {
		return nil, err
	}

	return namespace, nil
}

func (r *AddonReconciler) actualMonitoringNamespace(
	ctx context.Context, addon *addonsv1alpha1.Addon) (*corev1.Namespace, error) {
	key := client.ObjectKey{
		Name: GetMonitoringNamespaceName(addon),
	}

	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, key, namespace); err != nil {
		return nil, err
	}

	return namespace, nil
}

func (r *AddonReconciler) ensureServiceMonitor(ctx context.Context, addon *addonsv1alpha1.Addon) error {
	desired, err := r.desiredServiceMonitor(addon)
	if err != nil {
		return err
	}

	actual, err := r.actualServiceMonitor(ctx, addon)
	if k8sApiErrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	} else if err != nil {
		return fmt.Errorf("getting ServiceMonitor: %w", err)
	}

	var (
		mustAdopt   = !controllers.HasEqualControllerReference(actual, desired)
		specChanged = !equality.Semantic.DeepEqual(actual.Spec, desired.Spec)
	)

	if !mustAdopt && !specChanged {
		return nil
	}

	if mustAdopt && !HasAdoptAllStrategy(addon) {
		return controllers.ErrNotOwnedByUs
	}

	actual.Spec, actual.OwnerReferences = desired.Spec, desired.OwnerReferences

	return r.Update(ctx, actual)
}

func (r *AddonReconciler) desiredServiceMonitor(addon *addonsv1alpha1.Addon) (*monitoringv1.ServiceMonitor, error) {
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetMonitoringFederationServiceMonitorName(addon),
			Namespace: GetMonitoringNamespaceName(addon),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: GetMonitoringFederationServiceMonitorEndpoints(addon),
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{addon.Spec.Monitoring.Federation.Namespace},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: addon.Spec.Monitoring.Federation.MatchLabels,
			},
		},
	}

	controllers.AddCommonLabels(serviceMonitor.Labels, addon)

	if err := controllerutil.SetControllerReference(addon, serviceMonitor, r.Scheme); err != nil {
		return nil, fmt.Errorf("setting controller reference on ServiceMonitor: %w", err)
	}

	return serviceMonitor, nil
}

func (r *AddonReconciler) actualServiceMonitor(
	ctx context.Context, addon *addonsv1alpha1.Addon) (*monitoringv1.ServiceMonitor, error) {
	key := client.ObjectKey{
		Name:      GetMonitoringFederationServiceMonitorName(addon),
		Namespace: GetMonitoringNamespaceName(addon),
	}

	serviceMonitor := &monitoringv1.ServiceMonitor{}
	if err := r.Get(ctx, key, serviceMonitor); err != nil {
		return nil, err
	}

	return serviceMonitor, nil
}
