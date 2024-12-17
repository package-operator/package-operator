package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectSetPhase is an internal API, allowing an ObjectSet to delegate a single phase to another custom controller.
// ObjectSets will create subordinate ObjectSetPhases when `.class` within the phase specification is set.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={"objsetphase","osp"}
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=`.status.conditions[?(@.type=="Paused")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSetPhase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectSetPhaseSpec   `json:"spec,omitempty"`
	Status ObjectSetPhaseStatus `json:"status,omitempty"`
}

// ObjectSetPhaseList contains a list of ObjectSetPhases.
// +kubebuilder:object:root=true
type ObjectSetPhaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSetPhase `json:"items"`
}

// ObjectSetPhaseSpec defines the desired state of a ObjectSetPhase.
// +kubebuilder:validation:XValidation:rule="has(self.previous) == has(oldSelf.previous)", message="previous is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.availabilityProbes) == has(oldSelf.availabilityProbes)", message="availabilityProbes is immutable"
type ObjectSetPhaseSpec struct {
	// Disables reconciliation of the ObjectSet.
	// Only Status updates will still be propagated, but object changes will not be reconciled.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Immutable fields below

	// Revision of the parent ObjectSet to use during object adoption.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="revision is immutable"
	Revision int64 `json:"revision"`

	// Previous revisions of the ObjectSet to adopt objects from.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="previous is immutable"
	// +kubebuilder:MaxItems=32
	Previous []PreviousRevisionReference `json:"previous,omitempty"`

	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="availabilityProbes is immutable"
	// +kubebuilder:MaxItems=32
	AvailabilityProbes []ObjectSetProbe `json:"availabilityProbes,omitempty"`

	// Objects belonging to this phase.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="objects is immutable"
	// +kubebuilder:MaxItems=32
	Objects []ObjectSetObject `json:"objects"`
}

// ObjectSetPhaseStatus defines the observed state of a ObjectSetPhase.
type ObjectSetPhaseStatus struct {
	// Conditions is a list of status conditions ths object is in.
	// +example=[{type: "Available", status: "True"}]
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// References all objects controlled by this instance.
	ControllerOf []ControlledObjectReference `json:"controllerOf,omitempty"`
}

func init() { register(&ObjectSetPhase{}, &ObjectSetPhaseList{}) }
