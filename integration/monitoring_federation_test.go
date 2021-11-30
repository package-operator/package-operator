package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
	"github.com/openshift/addon-operator/internal/controllers"
)

func (s *integrationTestSuite) TestMonitoringFederation() {
	s.Run("MonitoringInPlaceAtCreationRemovedAfterwards", func() {
		ctx := context.Background()

		addon := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: "addon-41b95034425c4d55",
			},
			Spec: addonsv1alpha1.AddonSpec{
				DisplayName: "addon-41b95034425c4d55",
				Namespaces: []addonsv1alpha1.AddonNamespace{
					{Name: "namespace-a9953682ff70d594"},
				},
				Install: addonsv1alpha1.AddonInstallSpec{
					Type: addonsv1alpha1.OLMOwnNamespace,
					OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
						AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
							Namespace:          "namespace-a9953682ff70d594",
							CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
							Channel:            "alpha",
							PackageName:        "reference-addon",
						},
					},
				},
				Monitoring: &addonsv1alpha1.MonitoringSpec{
					Federation: &addonsv1alpha1.MonitoringFederationSpec{
						Namespace:  "namespace-a9953682ff70d594",
						MatchNames: []string{"some_timeseries"},
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
		}

		err := integration.Client.Create(ctx, addon)
		s.Require().NoError(err)

		// clean up addon resource in case it
		// was leaked because of a failed test
		s.T().Cleanup(func() {
			s.addonCleanup(addon, ctx)
		})

		// wait until Addon is available
		err = integration.WaitForObject(
			s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available",
			func(obj client.Object) (done bool, err error) {
				a := obj.(*addonsv1alpha1.Addon)
				return meta.IsStatusConditionTrue(
					a.Status.Conditions, addonsv1alpha1.Available), nil
			})
		s.Require().NoError(err)

		monitoringNamespaceName := controllers.GetMonitoringNamespaceName(addon)

		// validate monitoring Namespace
		currentMonitoringNamespace := &corev1.Namespace{}
		{
			err := integration.Client.Get(ctx, types.NamespacedName{
				Name: monitoringNamespaceName,
			}, currentMonitoringNamespace)
			s.Assert().NoError(err, "could not get monitoring Namespace %s", monitoringNamespaceName)
		}

		// validate ServiceMonitor
		validateMonitoringFederationServiceMonitor(s.T(), ctx, addon, monitoringNamespaceName)

		// unset addon.spec.monitoring.federation and update Addon object
		addon.Spec.Monitoring.Federation = nil
		{
			err := integration.Client.Update(ctx, addon)
			s.Require().NoError(err)
		}

		// wait until Addon is available again
		err = integration.WaitForObject(
			s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available again",
			func(obj client.Object) (done bool, err error) {
				a := obj.(*addonsv1alpha1.Addon)
				return integration.IsFreshStatusConditionTrue(a, addonsv1alpha1.Available), nil
			})
		s.Require().NoError(err)

		// wait until monitoring Namespace is gone (ServiceMonitor will be gone as well)
		{
			err := integration.WaitToBeGone(s.T(), time.Minute, currentMonitoringNamespace)
			s.Require().NoError(err)
		}
	})

	s.Run("MonitoringNotInPlaceAtCreationAddedAfterwards", func() {
		ctx := context.Background()

		addon := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: "addon-oe7phook",
			},
			Spec: addonsv1alpha1.AddonSpec{
				DisplayName: "addon-oe7phook",
				Namespaces: []addonsv1alpha1.AddonNamespace{
					{Name: "namespace-xoh2pa0l"},
				},
				Install: addonsv1alpha1.AddonInstallSpec{
					Type: addonsv1alpha1.OLMOwnNamespace,
					OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
						AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
							Namespace:          "namespace-xoh2pa0l",
							CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
							Channel:            "alpha",
							PackageName:        "reference-addon",
						},
					},
				},
			},
		}

		err := integration.Client.Create(ctx, addon)
		s.Require().NoError(err)

		// clean up addon resource in case it
		// was leaked because of a failed test
		s.T().Cleanup(func() {
			s.addonCleanup(addon, ctx)
		})

		// wait until Addon is available
		err = integration.WaitForObject(
			s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available",
			func(obj client.Object) (done bool, err error) {
				a := obj.(*addonsv1alpha1.Addon)
				return meta.IsStatusConditionTrue(
					a.Status.Conditions, addonsv1alpha1.Available), nil
			})
		s.Require().NoError(err)

		monitoringNamespaceName := controllers.GetMonitoringNamespaceName(addon)

		// validate that monitoring Namespace is not there
		{
			currentMonitoringNamespace := &corev1.Namespace{}
			err := integration.Client.Get(ctx, types.NamespacedName{
				Name: monitoringNamespaceName,
			}, currentMonitoringNamespace)
			s.Assert().Error(err, "getting a non-existent Namespace should error")
			s.Require().Equal(true, k8sApiErrors.IsNotFound(err), "error should have been 'Not Found'")
		}

		// set addon.spec.monitoring.federation and update Addon object
		addon.Spec.Monitoring = &addonsv1alpha1.MonitoringSpec{
			Federation: &addonsv1alpha1.MonitoringFederationSpec{
				Namespace:  "namespace-xoh2pa0l",
				MatchNames: []string{"some_timeseries"},
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		}

		{
			err := integration.Client.Update(ctx, addon)
			s.Require().NoError(err)
		}

		// wait until Addon is available again
		err = integration.WaitForObject(
			s.T(), defaultAddonAvailabilityTimeout, addon, "to be Available again",
			func(obj client.Object) (done bool, err error) {
				a := obj.(*addonsv1alpha1.Addon)
				return integration.IsFreshStatusConditionTrue(a, addonsv1alpha1.Available), nil
			})
		s.Require().NoError(err)

		// validate monitoring Namespace
		currentMonitoringNamespace := &corev1.Namespace{}
		{
			err := integration.Client.Get(ctx, types.NamespacedName{
				Name: monitoringNamespaceName,
			}, currentMonitoringNamespace)
			s.Assert().NoError(err, "could not get monitoring Namespace %s", monitoringNamespaceName)
		}

		// validate ServiceMonitor
		validateMonitoringFederationServiceMonitor(s.T(), ctx, addon, monitoringNamespaceName)
	})
}

func validateMonitoringFederationServiceMonitor(t *testing.T, ctx context.Context, addon *addonsv1alpha1.Addon, monitoringNamespaceName string) {
	serviceMonitorName := controllers.GetMonitoringFederationServiceMonitorName(addon)
	currentServiceMonitor := &monitoringv1.ServiceMonitor{}
	err := integration.Client.Get(ctx, types.NamespacedName{
		Name:      serviceMonitorName,
		Namespace: monitoringNamespaceName,
	}, currentServiceMonitor)
	require.NoError(t, err, "could not get monitoring federation ServiceMonitor %s", serviceMonitorName)
	assert.Equal(t, monitoringv1.ServiceMonitorSpec{
		Endpoints: []monitoringv1.Endpoint{
			{
				HonorLabels: true,
				Port:        "9090",
				Path:        "/federate",
				Scheme:      "https",
				Params: map[string][]string{
					"match[]": {
						`ALERTS{alertstate="firing"}`,
						`{__name__="some_timeseries"}`,
					},
				},
				Interval: "30s",
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
	}, currentServiceMonitor.Spec)
}
