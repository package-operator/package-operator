package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Package struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase: Pending}
	Status PackageStatus `json:"status,omitempty"`
}

// PackageList contains a list of Packages.
// +kubebuilder:object:root=true
type PackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Package `json:"items"`
}

func init() { register(&Package{}, &PackageList{}) }
