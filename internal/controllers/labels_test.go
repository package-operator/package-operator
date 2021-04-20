package controllers

import (
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

func TestAddCommonLabels(t *testing.T) {
	addon := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
	}

	labels := make(map[string]string)

	addCommonLabels(labels, addon)

	if labels[commonInstanceLabel] != addon.Name {
		t.Error("commonInstanceLabel was not set to addon name")
	}

	if labels[commonManagedByLabel] != commonManagedByValue {
		t.Error("commonManagedByLabel was not set to operator name")
	}
}

func TestCommonLabelsAsLabelSelector(t *testing.T) {
	addonWithCorrectName := &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
	}
	selector := commonLabelsAsLabelSelector(addonWithCorrectName)

	if selector.Empty() {
		t.Fatal("selector is empty but should filter on common labels")
	}
}
