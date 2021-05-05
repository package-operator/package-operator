package controllers

import (
	"k8s.io/apimachinery/pkg/labels"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

const (
	commonInstanceLabel  = "app.kubernetes.io/instance"
	commonManagedByLabel = "app.kubernetes.io/managed-by"
	commonManagedByValue = "addon-operator"
)

func addCommonLabels(labels map[string]string, addon *addonsv1alpha1.Addon) {
	if labels == nil {
		return
	}

	labels[commonManagedByLabel] = commonManagedByValue
	labels[commonInstanceLabel] = addon.Name
}

func commonLabelsAsLabelSelector(addon *addonsv1alpha1.Addon) labels.Selector {
	labelSet := make(labels.Set)
	labelSet[commonManagedByLabel] = commonManagedByValue
	labelSet[commonInstanceLabel] = addon.Name
	return labelSet.AsSelector()
}
