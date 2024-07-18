package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectSync synchronizes a singlular source object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName={"objsync","osync","os"}
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Src SyncedObjectReference `json:"src"`

	// +kubebuilder:validation:Required
	// +kubebuilder:MaxItems=32
	// +kubebuilder:validation:UniqueItems=true
	Dest []NamespacedName `json:"dest"`
}

// ObjectSyncList contains a list of ObjectSyncs.
// +kubebuilder:object:root=true
type ObjectSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSync `json:"items"`
}

// func init() { register(&ObjectSync{}, &ClusterObjectSliceList{}) }

// SyncedObjectReference an object synchronized by this ObjectSync.
type SyncedObjectReference struct {
	// Object Kind. Only ConfigMaps and Secrets allowed for now.
	// +kubebuilder:validation:Enum=ConfigMap;Secret
	Kind string `json:"kind"`
	// Object Name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Object Namespace.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// Wether source object should be marked with a label that allows PKO to cache the object.
	// Defaults to true but it can be toggled off in case there are conflicts with other controllers of the source object.
	// TODO: If this is turned off, a reconciliation interval has to be defined.
	// +kubebuilder:default=true
	Mark bool `json:"mark"`
}

// NamespacedName.
type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}
