package addon

import (
	"context"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestEnsureDeletionOfMonitoringFederation_MonitoringFullyMissingInSpec_NotPresentInCluster(t *testing.T) {
	c := testutil.NewClient()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "addon-foo",
		},
	}

	c.On("List", testutil.IsContext, mock.IsType(&monitoringv1.ServiceMonitorList{}), mock.Anything).
		Return(nil)
	c.On("Delete", testutil.IsContext, mock.IsType(&corev1.Namespace{}), mock.Anything).
		Run(func(args mock.Arguments) {
			ns := args.Get(1).(*corev1.Namespace)
			assert.Equal(t, controllers.GetMonitoringNamespaceName(addon), ns.Name)
		}).
		Return(testutil.NewTestErrNotFound())

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureDeletionOfUnwantedMonitoringFederation(ctx, addon)

	require.NoError(t, err)
	c.AssertExpectations(t)
}

func TestEnsureDeletionOfMonitoringFederation_MonitoringFullyMissingInSpec_PresentInCluster(t *testing.T) {
	c := testutil.NewClient()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "addon-foo",
		},
	}

	serviceMonitorsInCluster := &monitoringv1.ServiceMonitorList{
		Items: []*monitoringv1.ServiceMonitor{
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      "qux",
					Namespace: "bar",
				},
			},
		},
	}
	deletedServiceMons := []string{}

	c.On("List", testutil.IsContext, mock.IsType(&monitoringv1.ServiceMonitorList{}), mock.Anything).
		Run(func(args mock.Arguments) {
			list := args.Get(1).(*monitoringv1.ServiceMonitorList)
			serviceMonitorsInCluster.DeepCopyInto(list)
		}).
		Return(nil)
	c.On("Delete", testutil.IsContext, mock.IsType(&monitoringv1.ServiceMonitor{}), mock.Anything).
		Run(func(args mock.Arguments) {
			sm := args.Get(1).(*monitoringv1.ServiceMonitor)
			assert.Condition(t, func() (success bool) {
				for _, serviceMonitorInCluster := range serviceMonitorsInCluster.Items {
					if serviceMonitorInCluster.Name == sm.Name {
						return true
					}
				}
				return false
			})
			deletedServiceMons = append(deletedServiceMons, sm.Name)
		}).
		Return(nil)
	c.On("Delete", testutil.IsContext, mock.IsType(&corev1.Namespace{}), mock.Anything).
		Run(func(args mock.Arguments) {
			ns := args.Get(1).(*corev1.Namespace)
			assert.Equal(t, controllers.GetMonitoringNamespaceName(addon), ns.Name)
		}).
		Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureDeletionOfUnwantedMonitoringFederation(ctx, addon)

	require.NoError(t, err)
	c.AssertExpectations(t)
	c.AssertCalled(t, "Delete", testutil.IsContext, mock.IsType(&corev1.Namespace{}), mock.Anything)
	assert.Equal(t, []string{"foo", "qux"}, deletedServiceMons)
}

func TestEnsureDeletionOfMonitoringFederation_MonitoringFullyPresentInSpec_PresentInCluster(t *testing.T) {
	c := testutil.NewClient()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "addon-foo",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Monitoring: &addonsv1alpha1.MonitoringSpec{
				Federation: &addonsv1alpha1.MonitoringFederationSpec{
					Namespace:  "addon-foo-test-ns",
					MatchNames: []string{"foo"},
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}

	serviceMonitorsInCluster := &monitoringv1.ServiceMonitorList{
		Items: []*monitoringv1.ServiceMonitor{
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      controllers.GetMonitoringFederationServiceMonitorName(addon),
					Namespace: controllers.GetMonitoringNamespaceName(addon),
					Labels:    map[string]string{},
				},
			},
		},
	}
	controllers.AddCommonLabels(serviceMonitorsInCluster.Items[0].Labels, addon)

	c.On("List", testutil.IsContext, mock.IsType(&monitoringv1.ServiceMonitorList{}), mock.Anything).
		Run(func(args mock.Arguments) {
			list := args.Get(1).(*monitoringv1.ServiceMonitorList)
			serviceMonitorsInCluster.DeepCopyInto(list)
		}).
		Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureDeletionOfUnwantedMonitoringFederation(ctx, addon)

	require.NoError(t, err)
	c.AssertExpectations(t)
}
