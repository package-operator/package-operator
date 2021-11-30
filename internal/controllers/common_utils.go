package controllers

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

const (
	commonInstanceLabel  = "app.kubernetes.io/instance"
	commonManagedByLabel = "app.kubernetes.io/managed-by"
	commonManagedByValue = "addon-operator"
)

func AddCommonLabels(labels map[string]string, addon *addonsv1alpha1.Addon) {
	if labels == nil {
		return
	}

	labels[commonManagedByLabel] = commonManagedByValue
	labels[commonInstanceLabel] = addon.Name
}

func CommonLabelsAsLabelSelector(addon *addonsv1alpha1.Addon) labels.Selector {
	labelSet := make(labels.Set)
	labelSet[commonManagedByLabel] = commonManagedByValue
	labelSet[commonInstanceLabel] = addon.Name
	return labelSet.AsSelector()
}

// Tests if the controller reference on `wanted` matches the one on `current`
func HasEqualControllerReference(current, wanted metav1.Object) bool {
	currentOwnerRefs := current.GetOwnerReferences()

	var currentControllerRef *metav1.OwnerReference
	for _, ownerRef := range currentOwnerRefs {
		or := ownerRef
		if *or.Controller {
			currentControllerRef = &or
			break
		}
	}

	if currentControllerRef == nil {
		return false
	}

	wantedOwnerRefs := wanted.GetOwnerReferences()

	for _, ownerRef := range wantedOwnerRefs {
		// OwnerRef is the same if UIDs match
		if currentControllerRef.UID == ownerRef.UID {
			return true
		}
	}

	return false
}

// Helper function to compute monitoring Namespace name from addon object
func GetMonitoringNamespaceName(addon *addonsv1alpha1.Addon) string {
	return fmt.Sprintf("redhat-monitoring-%s", addon.Name)
}

// Helper function to compute monitoring federation ServiceMonitor name from addon object
func GetMonitoringFederationServiceMonitorName(addon *addonsv1alpha1.Addon) string {
	return fmt.Sprintf("federated-sm-%s", addon.Name)
}
