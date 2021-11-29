package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultAddonOperatorName = "addon-operator"
)

// AddonOperator condition reasons
const (
	// Addon operator is ready
	AddonOperatorReasonReady = "AddonOperatorReady"

	// Addon operator has paused reconciliation
	AddonOperatorReasonPaused = "AddonOperatorPaused"

	// Addon operator has resumed reconciliation
	AddonOperatorReasonUnpaused = "AddonOperatorUnpaused"
)

// AddonOperatorSpec defines the desired state of Addon operator.
type AddonOperatorSpec struct {
	// Pause reconciliation on all Addons in the cluster
	// when set to True
	// +optional
	Paused bool `json:"pause"`

	// OCM specific configuration.
	// Setting this subconfig will enable deeper OCM integration.
	// e.g. push status reporting, etc.
	// +optional
	OCM *AddonOperatorOCM `json:"ocm,omitempty"`
}

// OCM specific configuration.
type AddonOperatorOCM struct {
	// Root of the OCM API Endpoint.
	Endpoint string `json:"endpoint"`

	// Secret to authenticate to the OCM API Endpoint.
	// Only supports secrets of type "kubernetes.io/dockerconfigjson"
	// https://kubernetes.io/docs/concepts/configuration/secret/#secret-types
	Secret ClusterSecretReference `json:"secret"`
}

// AddonOperatorStatus defines the observed state of Addon
type AddonOperatorStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Timestamp of the last reported status check
	// +optional
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase AddonPhase `json:"phase,omitempty"`
}

const (
	// Available condition indicates that the AddonOperator alive and well.
	AddonOperatorAvailable = "Available"

	// Paused condition indicates that the AddonOperator is paused entirely.
	AddonOperatorPaused = "Paused"
)

// AddonOperator is the Schema for the AddonOperator API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type AddonOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AddonOperatorSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status AddonOperatorStatus `json:"status,omitempty"`
}

// AddonOperatorList contains a list of AddonOperators
// +kubebuilder:object:root=true
type AddonOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AddonOperator `json:"items"`
}

func init() {
	register(&AddonOperator{}, &AddonOperatorList{})
}
