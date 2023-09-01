package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
type PackageManifestLock struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageManifestLockSpec `json:"spec,omitempty"`
}

type PackageManifestLockSpec struct {
	// List of resolved images
	Images []PackageManifestLockImage `json:"images"`
	// Permissions required to install this package.
	// For every object that is part of the package get,list,create,update,patch,delete,watch verbs are required.
	// For external objects get,list,watch verbs are required.
	InstallPermissions []rbacv1.PolicyRule `json:"installPermissions"`
}

// PackageManifestLockImage contains information about a resolved image.
type PackageManifestLockImage struct {
	// Image name to be use to reference it in the templates
	Name string `json:"name"`
	// Image identifier (REPOSITORY[:TAG])
	Image string `json:"image"`
	// Image digest
	Digest string `json:"digest"`
}

func init() { register(&PackageManifestLock{}) }
