package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
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

	var deployments []unstructured.Unstructured

	// Create all objects to install the Addon Operator
	for _, obj := range objs {
		err := integration.Client.Create(ctx, &obj)
		require.NoError(t, err)

		t.Log("created: ", obj.GroupVersionKind().String(),
			obj.GetNamespace()+"/"+obj.GetName())

		if obj.GetKind() == "Deployment" {
			deployments = append(deployments, obj)
		}
	}

	crds := []struct {
		crdName string
		objList client.ObjectList
	}{
		{
			crdName: "addons.addons.managed.openshift.io",
			objList: &addonsv1alpha1.AddonList{},
		},
		{
			crdName: "addonoperators.addons.managed.openshift.io",
			objList: &addonsv1alpha1.AddonOperatorList{},
		},
	}

	for _, crd := range crds {
		crd := crd // pin
		t.Run(fmt.Sprintf("API %s established", crd.crdName), func(t *testing.T) {
			crdObj := &apiextensionsv1.CustomResourceDefinition{}

			err := wait.PollImmediate(time.Second, 10*time.Second, func() (done bool, err error) {
				err = integration.Client.Get(ctx, types.NamespacedName{
					Name: crd.crdName,
				}, crdObj)
				if err != nil {
					t.Logf("error getting CRD: %v", err)
					return false, nil
				}

				// check CRD Established Condition
				var establishedCond *apiextensionsv1.CustomResourceDefinitionCondition
				for _, c := range crdObj.Status.Conditions {
					if c.Type == apiextensionsv1.Established {
						establishedCond = &c
						break
					}
				}

				return establishedCond != nil && establishedCond.Status == apiextensionsv1.ConditionTrue, nil
			})
			require.NoError(t, err, "waiting for %s to be Established", crd.crdName)

			// check CRD API
			err = integration.Client.List(ctx, crd.objList)
			require.NoError(t, err)
		})
	}

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

	t.Run("Addon Operator available", func(t *testing.T) {
		addonOperator := addonsv1alpha1.AddonOperator{}

		// Wait for API to be created
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := integration.Client.Get(ctx, client.ObjectKey{
				Name: addonsv1alpha1.DefaultAddonOperator,
			}, &addonOperator)
			return err
		})
		require.NoError(t, err)

		err = integration.WaitForObject(
			t, defaultAddonAvailabilityTimeout, &addonOperator, "to be Available",
			func(obj client.Object) (done bool, err error) {
				a := obj.(*addonsv1alpha1.AddonOperator)
				return meta.IsStatusConditionTrue(
					a.Status.Conditions, addonsv1alpha1.Available), nil
			})
		require.NoError(t, err)
	})
}
