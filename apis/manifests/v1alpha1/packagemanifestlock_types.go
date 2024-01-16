package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PackageManifestLock allows locking packages to specific versions.
// +kubebuilder:object:root=true
type PackageManifestLock struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageManifestLockSpec `json:"spec,omitempty"`
}

// PackageManifestLockSpec defines a set of packages that are locked to a specific version.
type PackageManifestLockSpec struct {
	// List of resolved images
	Images []PackageManifestLockImage `json:"images"`
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
