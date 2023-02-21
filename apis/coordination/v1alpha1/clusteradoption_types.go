package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterAdoption assigns initial labels to objects using one of multiple strategies.
// e.g. to route them to a specific operator instance.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterAdoption struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterAdoptionSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ClusterAdoptionStatus `json:"status,omitempty"`
}

// ClusterAdoptionList contains a list of ClusterAdoptions.
// +kubebuilder:object:root=true
type ClusterAdoptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAdoption `json:"items"`
}

// ClusterAdoptionSpec defines the desired state of an ClusterAdoption.
type ClusterAdoptionSpec struct {
	// Strategy to use for adoption.
	Strategy ClusterAdoptionStrategy `json:"strategy"`
	// TargetAPI to use for adoption.
	TargetAPI TargetAPI `json:"targetAPI"`
}

// ClusterAdoptionStrategy defines the strategy to handover objects.
type ClusterAdoptionStrategy struct {
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

// ClusterAdoptionStatus defines the observed state of an ClusterAdoption.
type ClusterAdoptionStatus struct {
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
	SchemeBuilder.Register(&ClusterAdoption{}, &ClusterAdoptionList{})
}
