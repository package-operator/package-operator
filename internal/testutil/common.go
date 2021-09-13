package testutil

import (
	"fmt"
	"os"

	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"
)

// NewStatusError returns an error of type `StatusError `
func NewStatusError(msg string) *k8sApiErrors.StatusError {
	return &k8sApiErrors.StatusError{
		ErrStatus: v1.Status{
			Status: "Failure",
			Message: fmt.Sprintf("%s %s",
				"admission webhook \"vaddons.managed.openshift.io\" denied the request:",
				msg),
			Reason: v1.StatusReason(msg),
			Code:   403,
		},
	}
}

// NewAddonWithInstallSpec returns an Addon object with the specified InstallSpec
func NewAddonWithInstallSpec(installSpec addonsv1alpha1.AddonInstallSpec,
	addonName string) *addonsv1alpha1.Addon {
	return &addonsv1alpha1.Addon{
		ObjectMeta: v1.ObjectMeta{
			Name: addonName,
		},
		Spec: addonsv1alpha1.AddonSpec{
			DisplayName: "An example addon",
			Namespaces: []addonsv1alpha1.AddonNamespace{
				{Name: "reference-addon"},
			},
			Install: installSpec,
		},
	}
}

func IsWebhookServerEnabled() bool {
	value, exists := os.LookupEnv("ENABLE_WEBHOOK")
	return exists && value != "false"
}
