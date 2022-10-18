package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PackageStatus defines the observed state of a Package.
type PackageStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase PackageStatusPhase `json:"phase,omitempty"`
}

// Package condition types.
const (
	// A Packages "Available" condition tracks the availability of the underlying ObjectDeployment objects.
	// When the Package is reporting "Available" = True, it's expected that whatever the Package installs is up and operational.
	// Package "Availability" may change multiple times during it's lifecycle.
	PackageAvailable   = "Available"
	PackageProgressing = "Progressing"
	PackageUnpacked    = "Unpacked"
)

type PackageStatusPhase string

// Well-known Package Phases for printing a Status in kubectl,
// see deprecation notice in PackageStatus for details.
const (
	PackagePhasePending     PackageStatusPhase = "Pending"
	PackagePhaseAvailable   PackageStatusPhase = "Available"
	PackagePhaseProgressing PackageStatusPhase = "Progressing"
	PackagePhaseUnpacking   PackageStatusPhase = "Unpacking"
	PackagePhaseNotReady    PackageStatusPhase = "NotReady"
)

// Package specification.
type PackageSpec struct {
	// the image containing the contents of the package
	// this image will be unpacked by the package-loader to render the ObjectDeployment for propagating the installation of the package.
	// +kubebuilder:validation:Required
	Image string `json:"image"`
}
