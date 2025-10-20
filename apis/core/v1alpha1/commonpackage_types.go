package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// PackageStatus defines the observed state of a Package.
type PackageStatus struct {
	// Conditions is a list of status conditions ths object is in.
	// +example=[{type: "Available", status: "True", reason: "Available",  message: "Latest Revision is Available."}]
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Hash of image + config that was successfully unpacked.
	UnpackedHash string `json:"unpackedHash,omitempty"`
	// Package revision as reported by the ObjectDeployment.
	Revision int64 `json:"revision,omitempty"`
}

// Package condition types.
const (
	// A Packages "Available" condition tracks the availability of the underlying ObjectDeployment objects.
	// When the Package is reporting "Available" = True, it's expected that whatever the Package installs
	// is up and operational. Package "Availability" may change multiple times during it's lifecycle.
	PackageAvailable = "Available"
	// Progressing indicates that a new release is being rolled out.
	PackageProgressing = "Progressing"
	// Unpacked tracks the completion or failure of the image unpack operation.
	PackageUnpacked = "Unpacked"
	// Invalid condition tracks unrecoverable validation and loading issues of the Package.
	// A package might be invalid because of multiple reasons:
	// - Does not support the right scope -> Namespaced vs. Cluster
	// - Missing or malformed PackageManifest
	// - Malformed Yaml
	// - Issues resulting from the template process.
	PackageInvalid = "Invalid"
	PackagePaused  = "Paused"
)

// PackageStatusPhase defines a status phase of a package.
type PackageStatusPhase string

// Well-known Package Phases for printing a Status in kubectl,
// see deprecation notice in PackageStatus for details.
const (
	PackagePhasePending     PackageStatusPhase = "Pending"
	PackagePhaseAvailable   PackageStatusPhase = "Available"
	PackagePhaseProgressing PackageStatusPhase = "Progressing"
	PackagePhaseUnpacking   PackageStatusPhase = "Unpacking"
	PackagePhaseNotReady    PackageStatusPhase = "NotReady"
	PackagePhaseInvalid     PackageStatusPhase = "Invalid"
	PackagePhasePaused      PackageStatusPhase = "Paused"
)

// PackageSpec specifies a package.
type PackageSpec struct {
	// the image containing the contents of the package
	// this image will be unpacked by the package-loader to render
	// the ObjectDeployment for propagating the installation of the package.
	// +kubebuilder:validation:Required
	Image string `json:"image"`
	// Package configuration parameters.
	// +kubebuilder:pruning:PreserveUnknownFields
	Config *runtime.RawExtension `json:"config,omitempty"`
	// Desired component to deploy from multi-component packages.
	// +optional
	Component string `json:"component,omitempty"`
	// If Paused is true, the package and its children will not be reconciled.
	Paused bool `json:"paused,omitempty"`
}

// PackageTemplateSpec describes the data a package should have when created from a template.
type PackageTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the package.
	// +optional
	Spec PackageSpec `json:"spec,omitempty"`
}
