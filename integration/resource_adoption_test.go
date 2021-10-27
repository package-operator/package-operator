package integration_test

import (
	"context"
	"testing"
	"time"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
	"github.com/openshift/addon-operator/internal/controllers"
)

var (
	referenceAddonNamespace   = "reference-addon"
	referenceAddonName        = "reference-addon"
	referenceAddonDisplayName = "Reference Addon"
)

// taken from -
// https://gitlab.cee.redhat.com/service/managed-tenants-manifests/-/blob/c60fa3f0252d908b5f868994f8934d24bbaca5f4/stage/addon-reference-addon-SelectorSyncSet.yaml
func getReferenceAddonNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"openshift.io/node-selector": "",
			},
			Name: referenceAddonNamespace,
		},
	}
}

func getReferenceAddonCatalogSource() *operatorsv1alpha1.CatalogSource {
	return &operatorsv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referenceAddonName,
			Namespace: referenceAddonNamespace,
		},
		Spec: operatorsv1alpha1.CatalogSourceSpec{
			DisplayName: referenceAddonDisplayName,
			Image:       referenceAddonCatalogSourceImageWorking,
			Publisher:   "OSD Red Hat Addons",
			SourceType:  operatorsv1alpha1.SourceTypeGrpc,
		},
	}
}

func getReferenceAddonOperatorGroup() *operatorsv1.OperatorGroup {
	return &operatorsv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referenceAddonName,
			Namespace: referenceAddonNamespace,
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{referenceAddonNamespace},
		},
	}
}

func getReferenceAddonSubscription() *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      referenceAddonName,
			Namespace: referenceAddonNamespace},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          referenceAddonName,
			CatalogSourceNamespace: referenceAddonNamespace,
			Channel:                referenceAddonNamespace,
			Package:                referenceAddonName,
		},
	}
}

func TestResourceAdoption(t *testing.T) {
	requiredOLMObjects := []client.Object{
		getReferenceAddonNamespace(),
		getReferenceAddonCatalogSource(),
		getReferenceAddonOperatorGroup(),
		getReferenceAddonSubscription(),
	}

	ctx := context.Background()
	for _, obj := range requiredOLMObjects {
		obj := obj
		t.Logf("creating %s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		err := integration.Client.Create(ctx, obj)
		require.NoError(t, err)
	}

	addon := &addonsv1alpha1.Addon{
		ObjectMeta: metav1.ObjectMeta{
			Name: referenceAddonName,
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: referenceAddonName,
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{
					Name: referenceAddonNamespace,
				},
			},
			Install: addonsv1alpha1.AddonInstallSpec{
				Type: addonsv1alpha1.OLMOwnNamespace,
				OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
					AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
						Namespace:          referenceAddonNamespace,
						PackageName:        referenceAddonNamespace,
						Channel:            "alpha",
						CatalogSourceImage: referenceAddonCatalogSourceImageWorking,
					},
				},
			},
		},
	}
	t.Run("resource adoption strategy: Prevent", func(t *testing.T) {
		addon := addon.DeepCopy()
		addon.Spec.ResourceAdoptionStrategy = addonsv1alpha1.ResourceAdoptionPrevent

		err := integration.Client.Create(ctx, addon)
		require.NoError(t, err)

		observedAddon := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: referenceAddonName,
			},
		}

		// check status condition for collided namespace error
		err = integration.WaitForObject(
			t, 10*time.Minute, observedAddon, "to report collided namespaces",
			func(obj client.Object) (done bool, err error) {
				addon := obj.(*addonsv1alpha1.Addon)
				if collidedCondition := meta.FindStatusCondition(addon.Status.Conditions,
					addonsv1alpha1.Available); collidedCondition != nil {
					return collidedCondition.Status == metav1.ConditionFalse &&
						collidedCondition.Reason == addonsv1alpha1.AddonReasonCollidedNamespaces, nil
				}
				return false, nil
			})
		require.NoError(t, err)

		// delete addon
		err = integration.Client.Delete(ctx, addon)
		require.NoError(t, err, "delete addon")

		err = integration.WaitToBeGone(t, defaultAddonDeletionTimeout, addon)
		require.NoError(t, err, "wait for Addon to be deleted")

	})

	t.Run("resource adoption strategy: AdoptAll", func(t *testing.T) {
		addon := addon.DeepCopy()
		addon.Spec.ResourceAdoptionStrategy = addonsv1alpha1.ResourceAdoptionAdoptAll

		err := integration.Client.Create(ctx, addon)
		require.NoError(t, err)

		observedAddon := &addonsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: referenceAddonName,
			},
		}

		err = integration.WaitForObject(
			t, 10*time.Minute, observedAddon, "to be available",
			func(obj client.Object) (done bool, err error) {
				addon := obj.(*addonsv1alpha1.Addon)
				return meta.IsStatusConditionTrue(addon.Status.Conditions,
					addonsv1alpha1.Available), nil
			})
		require.NoError(t, err)

		// validate ownerReference on Namespace
		{
			observedNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: referenceAddonNamespace,
				},
			}
			err = integration.WaitForObject(
				t, 2*time.Minute, observedNs, "to have AddonOperator ownerReference",
				func(obj client.Object) (done bool, err error) {
					ns := obj.(*corev1.Namespace)
					return validateOwnerReference(addon, ns)
				})
			require.NoError(t, err)
		}

		// validate ownerReference on Subscription
		{
			observedSubscription := &operatorsv1alpha1.Subscription{
				ObjectMeta: metav1.ObjectMeta{
					Name:      referenceAddonName,
					Namespace: referenceAddonNamespace,
				},
			}
			err = integration.WaitForObject(
				t, 2*time.Minute, observedSubscription, "to have AddonOperator ownerReference",
				func(obj client.Object) (done bool, err error) {
					sub := obj.(*operatorsv1alpha1.Subscription)
					return validateOwnerReference(addon, sub)
				})
			require.NoError(t, err)

		}

		// validate ownerReference on OperatorGroup
		{
			observedOG := &operatorsv1.OperatorGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      referenceAddonName,
					Namespace: referenceAddonNamespace,
				},
			}
			err = integration.WaitForObject(
				t, 2*time.Minute, observedOG, "to have AddonOperator ownerReference",
				func(obj client.Object) (done bool, err error) {
					og := obj.(*operatorsv1.OperatorGroup)
					return validateOwnerReference(addon, og)
				})
			require.NoError(t, err)

		}
		// validate ownerReference on CatalogSource
		{
			observedCS := &operatorsv1alpha1.CatalogSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      referenceAddonName,
					Namespace: referenceAddonNamespace,
				},
			}
			err = integration.WaitForObject(
				t, 2*time.Minute, observedCS, "to have AddonOperator ownerReference",
				func(obj client.Object) (done bool, err error) {
					cs := obj.(*operatorsv1alpha1.CatalogSource)
					return validateOwnerReference(addon, cs)
				})
			require.NoError(t, err)

		}

		// delete addon
		// note that this now also deletes the OLM objects
		err = integration.Client.Delete(ctx, addon)
		require.NoError(t, err, "delete addon")

		err = integration.WaitToBeGone(t, defaultAddonDeletionTimeout, addon)
		require.NoError(t, err, "wait for Addon to be deleted")
	})
}

func validateOwnerReference(addon *addonsv1alpha1.Addon, obj metav1.Object) (bool, error) {
	ownedObject := &corev1.Namespace{}
	testScheme := runtime.NewScheme()
	_ = addonsv1alpha1.AddToScheme(testScheme)
	err := controllerutil.SetControllerReference(addon, ownedObject, testScheme)
	if err != nil {
		return false, err
	}
	return controllers.HasEqualControllerReference(obj, ownedObject), nil
}
