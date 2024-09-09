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
	// List of resolved dependency images.
	Dependencies []PackageManifestLockDependency `json:"dependencies,omitempty"`
}

// PackageManifestLockDependency defines a dependency of this package.
type PackageManifestLockDependency struct {
	// Image name to be use to reference it in the templates
	// +example=my-pkg
	Name string `json:"name"`
	// Image identifier (REPOSITORY[:TAG])
	// +example=quay.io/package-operator/remote-phase-package
	Image string `json:"image"`
	// Image digest
	// +example=sha256:00e48c32b3cdcf9e2c66467f2beb0ef33b43b54e2b56415db4ee431512c406ea
	Digest string `json:"digest"`
	// Version of the dependency that has been chosen.
	// +example=v1.12.3
	Version string `json:"version"`
}

// PackageManifestLockImage contains information about a resolved image.
type PackageManifestLockImage struct {
	// Image name to be use to reference it in the templates
	// +example=my-pkg
	Name string `json:"name"`
	// Image identifier (REPOSITORY[:TAG])
	// +example=quay.io/package-operator/remote-phase-package
	Image string `json:"image"`
	// Image digest
	// +example=sha256:00e48c32b3cdcf9e2c66467f2beb0ef33b43b54e2b56415db4ee431512c406ea
	Digest string `json:"digest"`
}

func init() { register(&PackageManifestLock{}) }
