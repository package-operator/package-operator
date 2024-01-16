package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterPackage defines a cluster scoped package installation.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=clpkg
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterPackage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase: Pending}
	Status PackageStatus `json:"status,omitempty"`
}

// ClusterPackageList contains a list of Packages.
// +kubebuilder:object:root=true
type ClusterPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPackage `json:"items"`
}

func init() { register(&ClusterPackage{}, &ClusterPackageList{}) }
