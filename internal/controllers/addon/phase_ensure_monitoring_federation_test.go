package addon

import (
	"context"
	"fmt"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/internal/controllers"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestEnsureMonitoringFederation_MonitoringFullyMissingInSpec_NotPresentInCluster(t *testing.T) {
	c := testutil.NewClient()

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-foo",
		},
	}

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	ctx := context.Background()
	err := r.ensureMonitoringFederation(ctx, addon)
	require.NoError(t, err)
	c.AssertExpectations(t)
}

func TestEnsureMonitoringFederation_MonitoringPresentInSpec_NotPresentInCluster(t *testing.T) {
	c := testutil.NewClient()

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-foo",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Monitoring: &addonsv1alpha1.MonitoringSpec{
				Federation: &addonsv1alpha1.MonitoringFederationSpec{
					Namespace:  "addon-foo-monitoring",
					MatchNames: []string{"foo"},
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}

	c.On("Get", testutil.IsContext, mock.IsType(types.NamespacedName{}), mock.IsType(&corev1.Namespace{}), mock.Anything).
		Return(testutil.NewTestErrNotFound())
	c.On("Create", testutil.IsContext, mock.IsType(&corev1.Namespace{}), mock.Anything).
		Run(func(args mock.Arguments) {
			// mocked Namespace is immediately active
			namespace := args.Get(1).(*corev1.Namespace)
			namespace.Status.Phase = corev1.NamespaceActive
			assert.Equal(t, GetMonitoringNamespaceName(addon), namespace.Name)
		}).
		Return(nil)
	c.On("Get", testutil.IsContext, mock.IsType(types.NamespacedName{}), mock.IsType(&monitoringv1.ServiceMonitor{})).
		Return(testutil.NewTestErrNotFound())
	c.On("Create", testutil.IsContext, mock.IsType(&monitoringv1.ServiceMonitor{}), mock.Anything).
		Run(func(args mock.Arguments) {
			serviceMonitor := args.Get(1).(*monitoringv1.ServiceMonitor)
			assert.Equal(t, GetMonitoringFederationServiceMonitorName(addon), serviceMonitor.Name)
			assert.Equal(t, GetMonitoringNamespaceName(addon), serviceMonitor.Namespace)
		}).
		Return(nil)

	ctx := context.Background()
	err := r.ensureMonitoringFederation(ctx, addon)
	require.NoError(t, err)
	c.AssertExpectations(t)
	c.AssertNumberOfCalls(t, "Get", 2)
	c.AssertNumberOfCalls(t, "Create", 2)
}

func TestEnsureMonitoringFederation_MonitoringPresentInSpec_PresentInCluster(t *testing.T) {
	c := testutil.NewClient()

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-foo",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Monitoring: &addonsv1alpha1.MonitoringSpec{
				Federation: &addonsv1alpha1.MonitoringFederationSpec{
					Namespace:  "addon-foo-monitoring",
					MatchNames: []string{"foo"},
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}

	c.On("Get", testutil.IsContext, mock.IsType(types.NamespacedName{}), mock.IsType(&corev1.Namespace{}), mock.Anything).
		Run(func(args mock.Arguments) {
			namespacedName := args.Get(1).(types.NamespacedName)
			assert.Equal(t, GetMonitoringNamespaceName(addon), namespacedName.Name)
			// mocked Namespace is immediately active
			namespace := args.Get(2).(*corev1.Namespace)
			namespace.Status.Phase = corev1.NamespaceActive
			// mocked Namespace is owned by Addon
			err := controllerutil.SetControllerReference(addon, namespace, r.Scheme)
			// mocked Namespace has desired labels
			namespace.Labels = map[string]string{"openshift.io/cluster-monitoring": "true"}
			controllers.AddCommonLabels(namespace.Labels, addon)
			assert.NoError(t, err)
		}).
		Return(nil)

	c.On("Get", testutil.IsContext, mock.IsType(types.NamespacedName{}), mock.IsType(&monitoringv1.ServiceMonitor{}), mock.Anything).
		Run(func(args mock.Arguments) {
			namespacedName := args.Get(1).(types.NamespacedName)
			assert.Equal(t, GetMonitoringFederationServiceMonitorName(addon), namespacedName.Name)
			assert.Equal(t, GetMonitoringNamespaceName(addon), namespacedName.Namespace)
			// mocked ServiceMonitor is owned by Addon
			serviceMonitor := args.Get(2).(*monitoringv1.ServiceMonitor)
			err := controllerutil.SetControllerReference(addon, serviceMonitor, r.Scheme)
			assert.NoError(t, err)
			// inject expected ServiceMonitor spec into response
			serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
				Endpoints: []monitoringv1.Endpoint{
					{
						HonorLabels: true,
						Port:        "9090",
						Path:        "/federate",
						Scheme:      "https",
						Params: map[string][]string{
							"match[]": {
								`ALERTS{alertstate="firing"}`,
								`{__name__="foo"}`,
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
			}
		}).
		Return(nil)

	ctx := context.Background()
	err := r.ensureMonitoringFederation(ctx, addon)
	require.NoError(t, err)
}

func TestEnsureMonitoringFederation_Adoption(t *testing.T) {
	addon := testAddonWithMonitoringFederation()

	for name, tc := range map[string]struct {
		ActualMonitoringNamespace *corev1.Namespace
		ActualServiceMonitor      *monitoringv1.ServiceMonitor
		Strategy                  addonsv1alpha1.ResourceAdoptionStrategyType
		Expected                  error
	}{
		"existing namespace with no owner/no strategy": {
			ActualMonitoringNamespace: testMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionStrategyType(""),
			Expected:                  controllers.ErrNotOwnedByUs,
		},
		"existing namespace with no owner/Prevent": {
			ActualMonitoringNamespace: testMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionPrevent,
			Expected:                  controllers.ErrNotOwnedByUs,
		},
		"existing namespace with no owner/AdoptAll": {
			ActualMonitoringNamespace: testMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionAdoptAll,
			Expected:                  nil,
		},
		"existing namespace addon owned/no strategy": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionStrategyType(""),
			Expected:                  nil,
		},
		"existing namespace addon owned/Prevent": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionPrevent,
			Expected:                  nil,
		},
		"existing namespace addon owned/AdoptAll": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionAdoptAll,
			Expected:                  nil,
		},
		"existing serviceMonitor with no owner/no strategy": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      testServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionStrategyType(""),
			Expected:                  controllers.ErrNotOwnedByUs,
		},
		"existing serviceMonitor with no owner/Prevent": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      testServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionPrevent,
			Expected:                  controllers.ErrNotOwnedByUs,
		},
		"existing serviceMonitor with no owner/AdoptAll": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      testServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionAdoptAll,
			Expected:                  nil,
		},
		"existing serviceMonitor addon owned/no strategy": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionStrategyType(""),
			Expected:                  nil,
		},
		"existing serviceMonitor addon owned/Prevent": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionPrevent,
			Expected:                  nil,
		},
		"existing serviceMonitor addon owned/AdoptAll": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitor(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionAdoptAll,
			Expected:                  nil,
		},
		"existing serviceMonitor with altered spec/no strategy": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      testServiceMonitorAlteredSpec(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionStrategyType(""),
			Expected:                  controllers.ErrNotOwnedByUs,
		},
		"existing serviceMonitor with altered spec/Prevent": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      testServiceMonitorAlteredSpec(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionPrevent,
			Expected:                  controllers.ErrNotOwnedByUs,
		},
		"existing serviceMonitor with altered spec/AdoptAll": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      testServiceMonitorAlteredSpec(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionAdoptAll,
			Expected:                  nil,
		},
		"existing serviceMonitor with altered spec and addon owned/no strategy": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitorAlteredSpec(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionStrategyType(""),
			Expected:                  nil,
		},
		"existing serviceMonitor with altered spec and addon owned/Prevent": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitorAlteredSpec(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionPrevent,
			Expected:                  nil,
		},
		"existing serviceMonitor with altered spec and addon owned/AdoptAll": {
			ActualMonitoringNamespace: addonOwnedTestMonitoringNamespace(addon),
			ActualServiceMonitor:      addonOwnedTestServiceMonitorAlteredSpec(addon),
			Strategy:                  addonsv1alpha1.ResourceAdoptionAdoptAll,
			Expected:                  nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			client := testutil.NewClient()
			client.
				On("Get",
					testutil.IsContext,
					mock.IsType(types.NamespacedName{}),
					testutil.IsCoreV1NamespacePtr,
					mock.Anything).
				Run(func(args mock.Arguments) {
					tc.ActualMonitoringNamespace.DeepCopyInto(args.Get(2).(*corev1.Namespace))
				}).
				Return(nil)

			client.
				On("Update",
					testutil.IsContext,
					testutil.IsCoreV1NamespacePtr,
					mock.Anything).
				Return(nil).
				Maybe()

			client.StatusMock.
				On("Update",
					testutil.IsContext,
					testutil.IsAddonsv1alpha1AddonPtr,
					mock.Anything).
				Return(nil).
				Maybe()

			client.
				On("Get",
					testutil.IsContext,
					mock.IsType(types.NamespacedName{}),
					testutil.IsMonitoringV1ServiceMonitorPtr,
					mock.Anything).
				Run(func(args mock.Arguments) {
					tc.ActualServiceMonitor.DeepCopyInto(args.Get(2).(*monitoringv1.ServiceMonitor))
				}).
				Return(nil).
				Maybe()

			client.
				On("Update",
					testutil.IsContext,
					testutil.IsMonitoringV1ServiceMonitorPtr,
					mock.Anything).
				Return(nil).
				Maybe()

			rec := &AddonReconciler{
				Client: client,
				Log:    testutil.NewLogger(t),
				Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
			}

			addon := addon.DeepCopy()
			addon.Spec.ResourceAdoptionStrategy = tc.Strategy

			err := rec.ensureMonitoringFederation(context.Background(), addon)
			assert.ErrorIs(t, err, tc.Expected)

			client.AssertExpectations(t)
		})
	}
}

func testAddonWithMonitoringFederation() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-foo",
			UID:  types.UID("addon-foo-id"),
		},
		Spec: addonsv1alpha1.AddonSpec{
			Monitoring: &addonsv1alpha1.MonitoringSpec{
				Federation: &addonsv1alpha1.MonitoringFederationSpec{
					Namespace:  "addon-foo-monitoring",
					MatchNames: []string{"foo"},
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}
}

func addonOwnedTestMonitoringNamespace(addon *addonsv1alpha1.Addon) *corev1.Namespace {
	ns := testMonitoringNamespace(addon)
	_ = controllerutil.SetControllerReference(addon, ns, testutil.NewTestSchemeWithAddonsv1alpha1())

	return ns
}

func testMonitoringNamespace(addon *addonsv1alpha1.Addon) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: GetMonitoringNamespaceName(addon),
		},
		Status: corev1.NamespaceStatus{
			Phase: corev1.NamespaceActive,
		},
	}
}

func addonOwnedTestServiceMonitor(addon *addonsv1alpha1.Addon) *monitoringv1.ServiceMonitor {
	sm := testServiceMonitor(addon)
	_ = controllerutil.SetControllerReference(addon, sm, testutil.NewTestSchemeWithAddonsv1alpha1())

	return sm
}

func addonOwnedTestServiceMonitorAlteredSpec(addon *addonsv1alpha1.Addon) *monitoringv1.ServiceMonitor {
	sm := testServiceMonitorAlteredSpec(addon)
	_ = controllerutil.SetControllerReference(addon, sm, testutil.NewTestSchemeWithAddonsv1alpha1())

	return sm
}

func testServiceMonitorAlteredSpec(addon *addonsv1alpha1.Addon) *monitoringv1.ServiceMonitor {
	serviceMonitor := testServiceMonitor(addon)
	serviceMonitor.Spec.SampleLimit = 10

	return serviceMonitor
}

func testServiceMonitor(addon *addonsv1alpha1.Addon) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
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
}
