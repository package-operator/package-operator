package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectSet reconciles a collection of objects through ordered phases and aggregates their status.
//
// ObjectSets behave similarly to Kubernetes ReplicaSets, by managing a collection of objects and
// being itself mostly immutable. This object type is able to suspend/pause reconciliation of
// specific objects to facilitate the transition between revisions.
//
// Archived ObjectSets may stay on the cluster, to store information about previous revisions.
//
// A Cluster-scoped version of this API is available as ClusterObjectSet.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={"objset","os"}
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=`.status.conditions[?(@.type=="Paused")].status`
// +kubebuilder:printcolumn:name="Archived",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Archived")].status`
// +kubebuilder:printcolumn:name="Succeeded",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Succeeded")].status`
// +kubebuilder:printcolumn:name="InTransition",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="InTransition")].status`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.revision`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectSetSpec   `json:"spec,omitempty"`
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
// +kubebuilder:validation:XValidation:rule="(has(self.previous) == has(oldSelf.previous)) && (!has(self.previous) || (self.previous == oldSelf.previous))", message="previous is immutable"
// +kubebuilder:validation:XValidation:rule="(has(self.phases) == has(oldSelf.phases)) && (!has(self.phases) || (self.phases == oldSelf.phases))", message="phases is immutable"
// +kubebuilder:validation:XValidation:rule="(has(self.availabilityProbes) == has(oldSelf.availabilityProbes)) && (!has(self.availabilityProbes) || (self.availabilityProbes == oldSelf.availabilityProbes))", message="availabilityProbes is immutable"
// +kubebuilder:validation:XValidation:rule="(has(self.successDelaySeconds) == has(oldSelf.successDelaySeconds)) && (!has(self.successDelaySeconds) || (self.successDelaySeconds == oldSelf.successDelaySeconds))", message="successDelaySeconds is immutable"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.revision) || (self.revision == oldSelf.revision)", message="revision is immutable"
type ObjectSetSpec struct {
	// Specifies the lifecycle state of the ObjectSet.
	// +kubebuilder:default="Active"
	// +kubebuilder:validation:Enum=Active;Paused;Archived
	LifecycleState ObjectSetLifecycleState `json:"lifecycleState,omitempty"`

	// Immutable fields below

	// Previous revisions of the ObjectSet to adopt objects from.
	Previous []PreviousRevisionReference `json:"previous,omitempty"`

	ObjectSetTemplateSpec `json:",inline"`

	// Computed revision number, monotonically increasing.
	// TODO: after soaking, update the validation rule to match the other ones.
	// Currently, the rule allows adding the revision field to existing ObjectSets
	// to phase in the new revision numbering approach.
	Revision int64 `json:"revision,omitempty"`
}

// ObjectSetStatus defines the observed state of a ObjectSet.
type ObjectSetStatus struct {
	// Conditions is a list of status conditions ths object is in.
	// +example=[{type: "Available", status: "True", reason: "Available",  message: "Latest Revision is Available."}]
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Deprecated: use .spec.revision instead
	Revision int64 `json:"revision,omitempty"`
	// Remote phases aka ObjectSetPhase objects.
	RemotePhases []RemotePhaseReference `json:"remotePhases,omitempty"`
	// References all objects controlled by this instance.
	ControllerOf []ControlledObjectReference `json:"controllerOf,omitempty"`
}

func init() { register(&ObjectSet{}, &ObjectSetList{}) }
