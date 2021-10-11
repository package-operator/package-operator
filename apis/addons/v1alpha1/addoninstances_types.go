package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddonInstanceSpec defines the desired state of Addon operator.
type AddonInstanceSpec struct {
	// Name of Kubernetes Secret resource containing the credentials to Sendgrid
	// +kubebuilder:default=10
	HeartbeatUpdatePeriod int64 `json:"heartbeatUpdatePeriod,omitempty"`
	// TODO: add more stuff here
}

type AddonInstancePhase string

// Well-known Addon Phases for printing a Status in kubectl,
// see deprecation notice in AddonStatus for details.
const (
	AddonInstancePhasePending     AddonInstancePhase = "Pending"
	AddonInstancePhaseReady       AddonInstancePhase = "Ready"
	AddonInstancePhaseTerminating AddonInstancePhase = "Terminating"
	AddonInstancePhaseError       AddonInstancePhase = "Error"
)

// AddonInstanceStatus defines the observed state of Addon
type AddonInstanceStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Timestamp of the last reported status check
	// +optional
	LastHeartbeatTime metav1.Time        `json:"lastHeartbeatTime"`
	Phase             AddonInstancePhase `json:"phase,omitempty"`
}

// AddonInstance is the Schema for the AddonInstance API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type AddonInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AddonInstanceSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status AddonInstanceStatus `json:"status,omitempty"`
}

// AddonInstanceList contains a list of AddonInstances
// +kubebuilder:object:root=true
type AddonInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AddonInstance `json:"items"`
}

const (
	DefaultAddonInstanceName = "addon-instance"
)

var (
	DefaultAddonInstanceSpec AddonInstanceSpec = AddonInstanceSpec{
		HeartbeatUpdatePeriod: int64(10 * time.Second),
		//TODO: add more stuff later on
	}
)

func init() {
	SchemeBuilder.Register(&AddonInstance{}, &AddonInstanceList{})
}
