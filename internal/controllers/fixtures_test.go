package controllers

import (
	"net/http"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilpointer "k8s.io/utils/pointer"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func newTestSchemeWithAddonsv1alpha1() *runtime.Scheme {
	testScheme := runtime.NewScheme()
	_ = addonsv1alpha1.AddToScheme(testScheme)
	return testScheme
}

func newTestAddonWithoutNamespace() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-1",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Namespaces: []addonsv1alpha1.AddonNamespace{},
		},
	}
}

func newTestAddonWithSingleNamespace() *addonsv1alpha1.Addon {
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

func newTestAddonWithMultipleNamespaces() *addonsv1alpha1.Addon {
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

func newTestNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "foo-apiVersion",
					Kind:       "foo-kind",
					Name:       "foo-name",
					UID:        "foo-uid",
					Controller: utilpointer.BoolPtr(true),
				},
			},
		},
	}
}

func newTestExistingNamespaceWithoutOwner() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-1",
		},
	}
}

func newTestExistingNamespaceWithOwner() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "foo-apiVersion-something-else",
					Kind:       "foo-kind-something-else",
					Name:       "foo-name-something-else",
					UID:        "foo-uid-something-else",
					Controller: utilpointer.BoolPtr(true),
				},
			},
		},
	}
}

func newTestErrNotFound() *k8sApiErrors.StatusError {
	return &k8sApiErrors.StatusError{
		ErrStatus: metav1.Status{
			Status: metav1.StatusFailure,
			Code:   http.StatusNotFound,
			Reason: metav1.StatusReasonNotFound,
		},
	}
}

func newTestCatalogSource() *operatorsv1alpha1.CatalogSource {
	return &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "catalogsource-pfsdboia",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "foo-apiVersion",
					Kind:       "foo-kind",
					Name:       "foo-name",
					UID:        "foo-uid",
					Controller: utilpointer.BoolPtr(true),
				},
			}},
	}
}

func newTestCatalogSourceWithoutOwner() *operatorsv1alpha1.CatalogSource {
	return &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "catalogsource-pfsdboia",
			Namespace: "default",
		},
	}
}

func newTestAddonWithCatalogSourceImage() *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: "addon-1",
			UID:  "addon-uid",
		},
		Spec: addonsv1alpha1.AddonSpec{
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OwnNamespace,
				OwnNamespace: &addonsv1alpha1.AddonInstallOwnNamespace{
					AddonInstallCommon: addonsv1alpha1.AddonInstallCommon{
						CatalogSourceImage: "quay.io/osd-addons/test:sha256:04864220677b2ed6244f2e0d421166df908986700647595ffdb6fd9ca4e5098a",
						Namespace:          "addon-1",
					},
				},
			},
		},
	}
}
