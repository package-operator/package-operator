package integration_test

import (
	"context"
	"fmt"
	"time"

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

func (s *integrationTestSuite) Setup() {
	ctx := context.Background()
	objs := integration.LoadObjectsFromDeploymentFiles(s.T())

	var deployments []unstructured.Unstructured

	// Create all objects to install the Addon Operator
	for _, obj := range objs {
		o := obj
		// check if object already exists
		var existingObj client.Object
		err := integration.Client.Get(ctx, client.ObjectKey{
			Namespace: o.GetNamespace(),
			Name:      o.GetName(),
		}, existingObj)

		if err != nil {
			s.T().Log("error not found:", fmt.Sprintf("err: %v and existingObj: %v", err, existingObj))
			// if not create one
			err = integration.Client.Create(ctx, &o)
			s.Require().NoError(err)

			s.T().Log("created: ", o.GroupVersionKind().String(),
				o.GetNamespace()+"/"+o.GetName())

			if o.GetKind() == "Deployment" {
				deployments = append(deployments, o)
			}
		} else {
			s.T().Log("found: ", o.GroupVersionKind().String(),
				o.GetNamespace()+"/"+o.GetName())
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
		{
			crdName: "addoninstances.addons.managed.openshift.io",
			objList: &addonsv1alpha1.AddonInstanceList{},
		},
	}

	for _, crd := range crds {
		crd := crd // pin
		s.Run(fmt.Sprintf("API %s established", crd.crdName), func() {
			crdObj := &apiextensionsv1.CustomResourceDefinition{}

			err := wait.PollImmediate(time.Second, 10*time.Second, func() (done bool, err error) {
				err = integration.Client.Get(ctx, types.NamespacedName{
					Name: crd.crdName,
				}, crdObj)
				if err != nil {
					s.T().Logf("error getting CRD: %v", err)
					return false, nil
				}

				// check CRD Established Condition
				var establishedCond *apiextensionsv1.CustomResourceDefinitionCondition
				for _, cond := range crdObj.Status.Conditions {
					c := cond
					if c.Type == apiextensionsv1.Established {
						establishedCond = &c
						break
					}
				}

				return establishedCond != nil && establishedCond.Status == apiextensionsv1.ConditionTrue, nil
			})
			s.Require().NoError(err, "waiting for %s to be Established", crd.crdName)

			// check CRD API
			err = integration.Client.List(ctx, crd.objList)
			s.Require().NoError(err)
		})
	}

	for _, deploy := range deployments {
		s.Run(fmt.Sprintf("Deployment %s available", deploy.GetName()), func() {

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
						//nolint:nilerr // retry on transient errors
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
			s.Require().NoError(err, "wait for Addon Operator Deployment")
		})
	}

	s.Run("AddonOperator available", func() {
		addonOperator := addonsv1alpha1.AddonOperator{}

		// Wait for API to be created
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err := integration.Client.Get(ctx, client.ObjectKey{
				Name: addonsv1alpha1.DefaultAddonOperatorName,
			}, &addonOperator)
			return err
		})
		s.Require().NoError(err)

		err = integration.WaitForObject(
			s.T(), defaultAddonAvailabilityTimeout, &addonOperator, "to be Available",
			func(obj client.Object) (done bool, err error) {
				a := obj.(*addonsv1alpha1.AddonOperator)
				return meta.IsStatusConditionTrue(
					a.Status.Conditions, addonsv1alpha1.Available), nil
			})
		s.Require().NoError(err)
	})

	s.Run("Patch AddonOperator with OCM mock configuration", func() {
		addonOperator := &addonsv1alpha1.AddonOperator{}
		if err := integration.Client.Get(ctx, client.ObjectKey{
			Name: addonsv1alpha1.DefaultAddonOperatorName,
		}, addonOperator); err != nil {
			s.T().Fatalf("get AddonOperator object: %v", err)
		}

		addonOperator.Spec.OCM = &addonsv1alpha1.AddonOperatorOCM{
			Endpoint: integration.OCMAPIEndpoint,
			Secret: addonsv1alpha1.ClusterSecretReference{
				Name:      "api-mock",
				Namespace: "api-mock",
			},
		}
		if err := integration.Client.Update(ctx, addonOperator); err != nil {
			s.T().Fatalf("patch AddonOperator object: %v", err)
		}
	})
}
