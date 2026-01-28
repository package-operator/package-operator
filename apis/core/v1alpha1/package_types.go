package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Package defines a namespaced package installation.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pkg
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Progressing",type=string,JSONPath=`.status.conditions[?(@.type=="Progressing")].status`
// +kubebuilder:printcolumn:name="Unpacked",type=string,JSONPath=`.status.conditions[?(@.type=="Unpacked")].status`
// +kubebuilder:printcolumn:name="Invalid",type=string,JSONPath=`.status.conditions[?(@.type=="Invalid")].status`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.revision`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Package struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageSpec   `json:"spec,omitempty"`
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
