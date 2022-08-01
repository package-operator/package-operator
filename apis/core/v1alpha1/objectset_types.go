package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectSet reconciles a collection of objects across ordered phases and aggregates their status.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ObjectSetSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ObjectSetStatus `json:"status,omitempty"`
}

// ObjectSetList contains a list of ObjectSets
// +kubebuilder:object:root=true
type ObjectSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSet `json:"items"`
}

// ObjectSetSpec defines the desired state of a ObjectSet.
type ObjectSetSpec struct {
	// Specifies the lifecycle state of the ObjectSet.
	// +kubebuilder:default="Active"
	// +kubebuilder:validation:Enum=Active;Paused;Archived
	LifecycleState ObjectSetLifecycleState `json:"lifecycleState,omitempty"`
	// Pause reconciliation of specific objects, while still reporting status.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`

	// Immutable fields below

	ObjectSetTemplateSpec `json:",inline"`
}

func init() {
	SchemeBuilder.Register(&ObjectSet{}, &ObjectSetList{})
}
