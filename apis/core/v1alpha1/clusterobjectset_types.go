package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterObjectSet reconciles a collection of objects through ordered phases and aggregates their status.
//
// ClusterObjectSets behave similarly to Kubernetes ReplicaSets, by managing a collection of objects and being itself mostly immutable.
// This object type is able to suspend/pause reconciliation of specific objects to facilitate the transition between revisions.
//
// Archived ClusterObjectSets may stay on the cluster, to store information about previous revisions.
//
// A Namespace-scoped version of this API is available as ObjectSet.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterObjectSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterObjectSetSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase: Pending}
	Status ClusterObjectSetStatus `json:"status,omitempty"`
}

// ClusterObjectSetList contains a list of ClusterObjectSets.
// +kubebuilder:object:root=true
type ClusterObjectSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterObjectSet `json:"items"`
}

// ClusterObjectSetSpec defines the desired state of a ClusterObjectSet.
type ClusterObjectSetSpec struct {
	// Specifies the lifecycle state of the ClusterObjectSet.
	// +kubebuilder:default="Active"
	// +kubebuilder:validation:Enum=Active;Paused;Archived
	LifecycleState ObjectSetLifecycleState `json:"lifecycleState,omitempty"`

	// Immutable fields below

	// Previous revisions of the ClusterObjectSet to adopt objects from.
	Previous []PreviousRevisionReference `json:"previous,omitempty"`

	ObjectSetTemplateSpec `json:",inline"`
}

// ClusterObjectSetStatus defines the observed state of a ClusterObjectSet.
type ClusterObjectSetStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Phase is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase ObjectSetStatusPhase `json:"phase,omitempty"`
	// Computed revision number, monotonically increasing.
	Revision int64 `json:"revision,omitempty"`
	// Remote phases aka ClusterObjectSetPhase objects.
	RemotePhases []RemotePhaseReference `json:"remotePhases,omitempty"`
	// References all objects controlled by this instance.
	ControllerOf []ControlledObjectReference `json:"controllerOf,omitempty"`
}

func init() { register(&ClusterObjectSet{}, &ClusterObjectSetList{}) }
