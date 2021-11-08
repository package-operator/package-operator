package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddonInstanceSpec defines the configuration to consider while taking AddonInstance-related decisions such as HeartbeatTimeouts
type AddonInstanceSpec struct {
	// The periodic rate at which heartbeats are expected to be received by the AddonInstance object
	// +kubebuilder:default="10s"
	HeartbeatUpdatePeriod metav1.Duration `json:"heartbeatUpdatePeriod,omitempty"`
}

// AddonInstanceStatus defines the observed state of Addon
type AddonInstanceStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Timestamp of the last reported status check
	// +optional
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime"`
}

// AddonInstance is the Schema for the AddonInstance API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Last Heartbeat",type="date",JSONPath=".status.lastHeartbeatTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type AddonInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonInstanceSpec   `json:"spec,omitempty"`
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

// AddonInstance Conditions
const (
	// AddonInstanceHealthy tracks the general health of an Addon.
	//
	// If false the service is degraded to a point that manual intervention is likely.
	// Higher level controllers are adviced to stop actions that might further worsen the state of the service. E.g. by delaying upgrades until the status is cleared.
	AddonInstanceHealthy = "addons.managed.openshift.io/Healthy"
)

func init() {
	SchemeBuilder.Register(&AddonInstance{}, &AddonInstanceList{})
}
