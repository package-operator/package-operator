package addoninstance

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

var heartbeatTimeoutCondition metav1.Condition = metav1.Condition{
	Type:    "addons.managed.openshift.io/Healthy",
	Status:  "Unknown",
	Reason:  "HeartbeatTimeout",
	Message: "Addon failed to send heartbeat.",
}

func parseTargetNamespaceFromAddon(addon addonsv1alpha1.Addon) (string, error) {
	var targetNamespace string
	switch addon.Spec.Install.Type {
	case addonsv1alpha1.OLMOwnNamespace:
		if addon.Spec.Install.OLMOwnNamespace == nil ||
			len(addon.Spec.Install.OLMOwnNamespace.Namespace) == 0 {
			// invalid/missing configuration
			return "", fmt.Errorf(".install.spec.olmOwmNamespace.namespace not found")
		}
		targetNamespace = addon.Spec.Install.OLMOwnNamespace.Namespace

	case addonsv1alpha1.OLMAllNamespaces:
		if addon.Spec.Install.OLMAllNamespaces == nil ||
			len(addon.Spec.Install.OLMAllNamespaces.Namespace) == 0 {
			// invalid/missing configuration
			return "", fmt.Errorf(".install.spec.olmAllNamespaces.namespace not found")
		}
		targetNamespace = addon.Spec.Install.OLMAllNamespaces.Namespace
	default:
		// ideally, this should never happen
		// but technically, it is possible to happen if validation webhook is turned off and CRD validation gets bypassed via the `--validate=false` argument
		return "", fmt.Errorf("unsupported install type found: %s", addon.Spec.Install.Type)
	}
	return targetNamespace, nil
}
