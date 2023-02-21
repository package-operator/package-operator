package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Adoption assigns initial labels to objects using one of multiple strategies.
// e.g. to route them to a specific operator instance.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Adoption struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AdoptionSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status AdoptionStatus `json:"status,omitempty"`
}

// AdoptionList contains a list of Adoptions.
// +kubebuilder:object:root=true
type AdoptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Adoption `json:"items"`
}

// AdoptionSpec defines the desired state of an Adoption.
type AdoptionSpec struct {
	// Strategy to use for adoption.
	Strategy AdoptionStrategy `json:"strategy"`
	// TargetAPI to use for adoption.
	TargetAPI TargetAPI `json:"targetAPI"`
}

// AdoptionStrategy defines the strategy to handover objects.
type AdoptionStrategy struct {
	// Type of adoption strategy. Can be "Static", "RoundRobin".
	// +kubebuilder:default=Static
	// +kubebuilder:validation:Enum={"Static","RoundRobin"}
	Type AdoptionStrategyType `json:"type"`

	// Static adoption strategy configuration.
	// Only present when type=Static.
	Static *AdoptionStrategyStaticSpec `json:"static,omitempty"`

	// RoundRobin adoption strategy configuration.
	// Only present when type=RoundRobin.
	RoundRobin *AdoptionStrategyRoundRobinSpec `json:"roundRobin,omitempty"`
}

// AdoptionStatus defines the observed state of an Adoption.
type AdoptionStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase AdoptionPhase `json:"phase,omitempty"`
	// Tracks round robin state to restart where the last operation ended.
	RoundRobin *AdoptionRoundRobinStatus `json:"roundRobin,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Adoption{}, &AdoptionList{})
}
