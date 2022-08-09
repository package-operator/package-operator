package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterObjectSetPhase is an internal API, allowing an ClusterObjectSet to delegate a single phase to another custom controller.
// ClusterObjectSets will create subordinate ClusterObjectSetPhases when `.class` within the phase specification is set.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterObjectSetPhase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterObjectSetPhaseSpec   `json:"spec,omitempty"`
	Status ClusterObjectSetPhaseStatus `json:"status,omitempty"`
}

// ClusterObjectSetPhaseList contains a list of ClusterObjectSetPhases.
// +kubebuilder:object:root=true
type ClusterObjectSetPhaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterObjectSetPhase `json:"items"`
}

// ClusterObjectSetPhaseSpec defines the desired state of a ClusterObjectSetPhase.
type ClusterObjectSetPhaseSpec struct {
	// Paused disables reconciliation of the ClusterObjectSetPhase,
	// only Status updates will be propagated.
	// +example=true
	Paused bool `json:"paused,omitempty"`
	// Pause reconciliation of specific objects.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	AvailabilityProbes []ObjectSetProbe `json:"availabilityProbes"`

	// Immutable fields below

	ObjectSetTemplatePhase `json:",inline"`
}

// ClusterObjectSetPhaseStatus defines the observed state of a ClusterObjectSetPhase.
type ClusterObjectSetPhaseStatus struct {
	// Conditions is a list of status conditions ths object is in.
	// +example=[{type: "Available", status: "True"}]
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// List of objects the controller has paused reconciliation on.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ClusterObjectSetPhase{}, &ClusterObjectSetPhaseList{})
}
