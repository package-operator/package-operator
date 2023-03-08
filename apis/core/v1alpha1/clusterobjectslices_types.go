package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterObjectSlices are referenced by ObjectSets or ObjectDeployments and contain objects to
// limit the size of ObjectSet and ObjectDeployments when big packages are installed.
// This is necessary to work around the etcd object size limit of ~1.5MiB and to reduce load on the kube-apiserver.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterObjectSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Objects []ObjectSetObject `json:"objects"`
}

// ClusterObjectSliceList contains a list of ClusterObjectSlices.
// +kubebuilder:object:root=true
type ClusterObjectSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterObjectSlice `json:"items"`
}

func init() { register(&ClusterObjectSlice{}, &ClusterObjectSliceList{}) }
