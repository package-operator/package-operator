package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pkoapisv1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// +kubebuilder:object:root=true
type PackageManifest struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`

	Spec PackageManifestSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// PackageManifestSpec represents the spec of the packagemanifest containing the details about phases and availability probes
type PackageManifestSpec struct {
	// Phases correspond to the references to the phases which are going to be the part of the ObjectDeployment/ClusterObjectDeployment.
	Phases []PackageManifestPhase `json:"phases" yaml:"phases"`
	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	AvailabilityProbes []pkoapisv1alpha1.ObjectSetProbe `json:"availabilityProbes" yaml:"availabilityProbes"`
}

type PackageManifestPhase struct {
	// Name of the reconcile phase. Must be unique within a PackageManifest
	Name  string `json:"name" yaml:"name"`
	Class string `json:"class,omitempty" yaml:"class,omitempty"`
}

func init() {
	SchemeBuilder.Register(&PackageManifest{})
}
