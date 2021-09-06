package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
	"github.com/openshift/addon-operator/integration"
)

func Setup(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	objs := integration.LoadObjectsFromDeploymentFiles(t)

	var (
		deployments    []unstructured.Unstructured
		services       []unstructured.Unstructured
		webhookConfigs []unstructured.Unstructured
	)

	// Create all objects to install the Addon Operator
	for _, obj := range objs {
		err := integration.Client.Create(ctx, &obj)
		require.NoError(t, err)

		t.Log("created: ", obj.GroupVersionKind().String(),
			obj.GetNamespace()+"/"+obj.GetName())

		switch obj.GetKind() {
		case "Deployment":
			deployments = append(deployments, obj)
		case "Service":
			services = append(services, obj)
		case "ValidatingWebhookConfiguration":
			webhookConfigs = append(webhookConfigs, obj)
		default:
			continue
		}
	}

	t.Run("API available", func(t *testing.T) {
		addonCRD := &apiextensionsv1.CustomResourceDefinition{}
		err := wait.PollImmediate(time.Second, 10*time.Second, func() (done bool, err error) {
			err = integration.Client.Get(ctx, types.NamespacedName{
				Name: "addons.addons.managed.openshift.io",
			}, addonCRD)
			if err != nil {
				t.Logf("error getting Addons CRD: %v", err)
				return false, nil
			}

			// check CRD Established Condition
			var establishedCond *apiextensionsv1.CustomResourceDefinitionCondition
			for _, c := range addonCRD.Status.Conditions {
				if c.Type == apiextensionsv1.Established {
					establishedCond = &c
					break
				}
			}

			return establishedCond != nil && establishedCond.Status == apiextensionsv1.ConditionTrue, nil
		})
		require.NoError(t, err, "waiting for Addons CRD to be Established")

		// check CRD API
		addonList := &addonsv1alpha1.AddonList{}
		err = integration.Client.List(ctx, addonList)
		require.NoError(t, err)
	})

	for _, deploy := range deployments {
		t.Run(fmt.Sprintf("Deployment %s available", deploy.GetName()), func(t *testing.T) {

			deployment := &appsv1.Deployment{}
			err := wait.PollImmediate(
				time.Second, 5*time.Minute, func() (done bool, err error) {
					err = integration.Client.Get(
						ctx, client.ObjectKey{
							Name:      deploy.GetName(),
							Namespace: deploy.GetNamespace(),
						}, deployment)
					if errors.IsNotFound(err) {
						return false, err
					}
					if err != nil {
						// retry on transient errors
						return false, nil
					}

					for _, cond := range deployment.Status.Conditions {
						if cond.Type == appsv1.DeploymentAvailable &&
							cond.Status == corev1.ConditionTrue {
							return true, nil
						}
					}
					return false, nil
				})
			require.NoError(t, err, "wait for Addon Operator Deployment")
		})
	}

	// This test ensures that the Webhook Service and ValidatingWebhookConfiguration are
	// actually present in your cluster and ready to accept webhook requests.
	// Without this, you'd see a lot of flakiness in webhook_test.go
	t.Run("Webhook Available", func(t *testing.T) {

		for _, wc := range webhookConfigs {
			webhookObj := admissionv1.ValidatingWebhookConfiguration{}
			err := wait.PollImmediate(
				time.Second, 5*time.Minute, func() (done bool, err error) {
					err = integration.Client.Get(ctx, types.NamespacedName{
						Name: wc.GetName(),
					}, &webhookObj)

					if err != nil {
						return false, err
					}
					return true, nil
				})
			require.NoError(t, err, "wait for ValidatingWebhookConfiguration")
		}

		for _, svc := range services {
			webhookSvc := v1.Service{}
			err := wait.PollImmediate(
				time.Second, 5*time.Minute, func() (done bool, err error) {
					err = integration.Client.Get(ctx, types.NamespacedName{
						Name:      svc.GetName(),
						Namespace: svc.GetNamespace(),
					}, &webhookSvc)

					if err != nil {
						return false, err
					}

					return true, nil
				})
			require.NoError(t, err, "wait for Service")
		}

		testAddon := newAddonWithInstallSpec(addonsv1alpha1.AddonInstallSpec{
			Type: addonsv1alpha1.OLMOwnNamespace,
			OLMOwnNamespace: &addonsv1alpha1.AddonInstallOLMOwnNamespace{
				AddonInstallOLMCommon: addonsv1alpha1.AddonInstallOLMCommon{
					Namespace:          "reference-addon",
					PackageName:        ADDON_NAME,
					Channel:            "alpha",
					CatalogSourceImage: CATALOG_SOURCE_URL,
				},
			},
		})

		err := wait.PollImmediate(
			time.Second, 5*time.Minute, func() (done bool, err error) {
				err = integration.Client.Create(ctx, testAddon)

				if err != nil {
					return false, nil
				}

				return true, nil
			})
		require.NoError(t, err, "wait for Addon to be created")

		// cleanup
		err = integration.Client.Delete(ctx, testAddon)
		require.NoError(t, err)

		err = integration.WaitToBeGone(t, 5*time.Minute, testAddon)
		require.NoError(t, err, "wait for Addon to be deleted")
	})
}
