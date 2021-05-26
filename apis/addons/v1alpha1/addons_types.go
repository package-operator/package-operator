package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddonSpec defines the desired state of Addon.
type AddonSpec struct {
	// Human readable name for this addon.
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// Defines a list of Kubernetes Namespaces that belong to this Addon.
	// Namespaces listed here will be created prior to installation of the Addon and
	// will be removed from the cluster when the Addon is deleted.
	// Collisions with existing Namespaces are NOT allowed.
	Namespaces []AddonNamespace `json:"namespaces,omitempty"`

	// Defines how an Addon is installed.
	// This field is immutable.
	// TODO: enforce immutablity in webhook
	Install AddonInstallSpec `json:"install"`
}

// AddonInstallSpec defines the desired Addon installation type.
type AddonInstallSpec struct {
	// Type of installation.
	// +kubebuilder:validation:Enum={"OwnNamespace","AllNamespaces"}
	Type AddonInstallType `json:"type"`
	// AllNamespaces config parameters. Present only if Type = AllNamespaces.
	AllNamespaces *AddonInstallAllNamespaces `json:"allNamespaces,omitempty"`
	// OwnNamespace config parameters. Present only if Type = OwnNamespace.
	OwnNamespace *AddonInstallOwnNamespace `json:"ownNamespace,omitempty"`
}

// Common Addon installation parameters.
type AddonInstallCommon struct {
	// Namespace to install the Addon into.
	Namespace string `json:"namespace"`

	// Defines the CatalogSource image.
	// Please only use hashes and no tags here!
	// +kubebuilder:validation:MinLength=1
	CatalogSourceImage string `json:"catalogSourceImage"`
}

// AllNamespaces specific Addon installation parameters.
type AddonInstallAllNamespaces struct {
	AddonInstallCommon `json:",inline"`
}

// OwnNamespace specific Addon installation parameters.
type AddonInstallOwnNamespace struct {
	AddonInstallCommon `json:",inline"`
}

type AddonInstallType string

const (
	// All namespaces on the cluster (default)
	// installs the Operator in the default openshift-operators namespace to
	// watch and be made available to all namespaces in the cluster.
	// Maps directly to the OLM default install mode "all namespaces".
	AllNamespaces AddonInstallType = "AllNamespaces"
	// Installs the operator into a specific namespace.
	// The Operator will only watch and be made available for use in this single namespace.
	// Maps directly to the OLM install mode "specific namespace"
	OwnNamespace AddonInstallType = "OwnNamespace"
)

type AddonNamespace struct {
	// Name of the KubernetesNamespace.
	Name string `json:"name"`
}

const (
	// Available condition indicates that all resources for the Addon are reconciled and healthy
	Available = "Available"
)

// AddonStatus defines the observed state of Addon
type AddonStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase AddonPhase `json:"phase,omitempty"`
}

type AddonPhase string

// Well-known Addon Phases for printing a Status in kubectl,
// see deprecation notice in AddonStatus for details.
const (
	PhasePending     AddonPhase = "Pending"
	PhaseReady       AddonPhase = "Ready"
	PhaseTerminating AddonPhase = "Terminating"
	PhaseError       AddonPhase = "Error"
)

// Addon is the Schema for the Addons API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AddonSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status AddonStatus `json:"status,omitempty"`
}

// AddonList contains a list of Addon
// +kubebuilder:object:root=true
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}
