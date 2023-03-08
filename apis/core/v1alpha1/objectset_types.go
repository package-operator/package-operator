package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectSet reconciles a collection of objects through ordered phases and aggregates their status.
//
// ObjectSets behave similarly to Kubernetes ReplicaSets, by managing a collection of objects and being itself mostly immutable.
// This object type is able to suspend/pause reconciliation of specific objects to facilitate the transition between revisions.
//
// Archived ObjectSets may stay on the cluster, to store information about previous revisions.
//
// A Cluster-scoped version of this API is available as ClusterObjectSet.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ObjectSetSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase: Pending}
	Status ObjectSetStatus `json:"status,omitempty"`
}

// ObjectSetList contains a list of ObjectSets.
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

	// Immutable fields below

	// Previous revisions of the ObjectSet to adopt objects from.
	Previous []PreviousRevisionReference `json:"previous,omitempty"`

	ObjectSetTemplateSpec `json:",inline"`
}

// ObjectSetStatus defines the observed state of a ObjectSet.
type ObjectSetStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Phase is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase ObjectSetStatusPhase `json:"phase,omitempty"`
	// Computed revision number, monotonically increasing.
	Revision int64 `json:"revision,omitempty"`
	// Remote phases aka ObjectSetPhase objects.
	RemotePhases []RemotePhaseReference `json:"remotePhases,omitempty"`
	// References all objects controlled by this instance.
	ControllerOf []ControlledObjectReference `json:"controllerOf,omitempty"`
}

func init() { register(&ObjectSet{}, &ObjectSetList{}) }
