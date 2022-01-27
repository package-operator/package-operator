package testutil

import (
	"net/http"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilpointer "k8s.io/utils/pointer"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func NewTestSchemeWithAddonsv1alpha1() *runtime.Scheme {
	testScheme := runtime.NewScheme()
	_ = addonsv1alpha1.AddToScheme(testScheme)
	return testScheme
}

func NewTestAddonWithoutNamespace() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-1",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Namespaces: []addonsv1alpha1.AddonNamespace{},
		},
	}
}

func NewTestAddonWithSingleNamespace() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-1",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "namespace-1"},
			},
		},
	}
}

func NewTestAddonWithMultipleNamespaces() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-1",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "namespace-1"},
				{Name: "namespace-2"},
			},
		},
	}
}

func NewTestNamespace() *corev1.Namespace {
	ns := NewTestNamespaceWithoutOwner()
	ns.OwnerReferences = testOwnerRefs()

	return ns
}

func NewTestNamespaceWithoutOwner() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-1",
		},
	}
}

func NewTestExistingNamespace() *corev1.Namespace {
	ns := NewTestExistingNamespaceWithoutOwner()
	ns.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "foo-apiVersion-something-else",
			Kind:       "foo-kind-something-else",
			Name:       "foo-name-something-else",
			UID:        "foo-uid-something-else",
			Controller: utilpointer.BoolPtr(true),
		},
	}

	return ns
}

func NewTestExistingNamespaceWithoutOwner() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-1",
		},
	}
}

func NewTestErrNotFound() *k8sApiErrors.StatusError {
	return &k8sApiErrors.StatusError{
		ErrStatus: metav1.Status{
			Status: metav1.StatusFailure,
			Code:   http.StatusNotFound,
			Reason: metav1.StatusReasonNotFound,
		},
	}
}

func NewTestCatalogSource() *operatorsv1alpha1.CatalogSource {
	cs := NewTestCatalogSourceWithoutOwner()
	cs.OwnerReferences = testOwnerRefs()

	return cs
}

func NewTestCatalogSourceWithoutOwner() *operatorsv1alpha1.CatalogSource {
	return &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "catalogsource-pfsdboia",
			Namespace: "default",
		},
	}
}

func NewTestOperatorGroup() *operatorsv1.OperatorGroup {
	og := NewTestOperatorGroupWithoutOwner()
	og.OwnerReferences = testOwnerRefs()

	return og
}

func NewTestOperatorGroupWithoutOwner() *operatorsv1.OperatorGroup {
	return &operatorsv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testing",
			Namespace: "testing-ns",
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{"testing-ns"},
		},
	}
}

func NewTestAddonWithCatalogSourceImage() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-1",
			UID:  "addon-uid",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						CatalogSourceImage: "quay.io/osd-addons/test:sha256:04864220677b2ed6244f2e0d421166df908986700647595ffdb6fd9ca4e5098a",
						Namespace:          "addon-1",
					},
				},
			},
		},
	}
}

func NewTestAddonWithCatalogSourceImageWithResourceAdoptionStrategy(strategy addonsv1alpha1.ResourceAdoptionStrategyType) *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-1",
			UID:  "addon-uid",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						CatalogSourceImage: "quay.io/osd-addons/test:sha256:04864220677b2ed6244f2e0d421166df908986700647595ffdb6fd9ca4e5098a",
						Namespace:          "addon-1",
					},
				},
			},
			ResourceAdoptionStrategy: strategy,
		},
	}
}

func NewTestServiceMonitor() *monitoringv1.ServiceMonitor {
	sm := NewTestServiceMonitorWithoutOwner()
	sm.OwnerReferences = testOwnerRefs()

	return sm
}

func NewTestServiceMonitorWithoutOwner() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "servicemonitor-abcdefgh",
			Namespace: "default",
		},
	}
}

func NewTestSubscription() *operatorsv1alpha1.Subscription {
	sub := NewTestSubscriptionWithoutOwner()
	sub.OwnerReferences = testOwnerRefs()

	return sub
}

func NewTestSubscriptionWithoutOwner() *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-abcdefgh",
			Namespace: "default",
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{},
	}
}

func testOwnerRefs() []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			APIVersion: "foo-apiVersion",
			Kind:       "foo-kind",
			Name:       "foo-name",
			UID:        "foo-uid",
			Controller: utilpointer.BoolPtr(true),
		},
	}
}
