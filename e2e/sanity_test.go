package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// Test that the Addon CRD exists and is served.
func TestAddonAPIAvailable(t *testing.T) {
	ctx := context.Background()

	addonCRD := &apiextensionsv1.CustomResourceDefinition{}
	err := c.Get(ctx, types.NamespacedName{
		Name: "addons.addons.managed.openshift.io",
	}, addonCRD)
	require.NoError(t, err)

	// check CRD Established Condition
	var establishedCond *apiextensionsv1.CustomResourceDefinitionCondition
	for _, c := range addonCRD.Status.Conditions {
		if c.Type == apiextensionsv1.Established {
			establishedCond = &c
			break
		}
	}
	if assert.NotNil(t, establishedCond, "Established Condition not reported") {
		assert.Equal(t, apiextensionsv1.ConditionTrue, establishedCond.Status)
	}

	// check CRD API
	addonList := &addonsv1alpha1.AddonList{}
	err = c.List(ctx, addonList)
	require.NoError(t, err)
}
