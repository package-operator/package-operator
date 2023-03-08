package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectSlices are referenced by ObjectSets or ObjectDeployments and contain objects to
// limit the size of ObjectSets and ObjectDeployments when big packages are installed.
// This is necessary to work around the etcd object size limit of ~1.5MiB and to reduce load on the kube-apiserver.
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Objects []ObjectSetObject `json:"objects"`
}

// ObjectSliceList contains a list of ObjectSlices.
// +kubebuilder:object:root=true
type ObjectSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSlice `json:"items"`
}

func init() { register(&ObjectSlice{}, &ObjectSliceList{}) }
