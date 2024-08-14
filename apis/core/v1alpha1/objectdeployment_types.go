package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectDeploymentSpec defines the desired state of a ObjectDeployment.
type ObjectDeploymentSpec struct {
	// Number of old revisions in the form of archived ObjectSets to keep.
	// +kubebuilder:default=10
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
	// Selector targets ObjectSets managed by this Deployment.
	Selector metav1.LabelSelector `json:"selector"`
	// Template to create new ObjectSets from.
	Template ObjectSetTemplate `json:"template"`
}

// ObjectSetTemplate describes the template to create new ObjectSets from.
type ObjectSetTemplate struct {
	// Common Object Metadata.
	Metadata metav1.ObjectMeta `json:"metadata"`
	// ObjectSet specification.
	Spec ObjectSetTemplateSpec `json:"spec"`
}

// ObjectDeploymentStatus defines the observed state of a ObjectDeployment.
type ObjectDeploymentStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase ObjectDeploymentPhase `json:"phase,omitempty"`
	// Count of hash collisions of the ObjectDeployment.
	CollisionCount *int32 `json:"collisionCount,omitempty"`
	// Computed TemplateHash.
	TemplateHash string `json:"templateHash,omitempty"`
	// Deployment revision.
	Revision int64 `json:"revision,omitempty"`
	// References (Cluster)ObjectSets controlled by this instance.
	ControllerOf []ControlledObjectReference `json:"controllerOf,omitempty"`
}

// ObjectDeployment Condition Types.
const (
	ObjectDeploymentAvailable   = "Available"
	ObjectDeploymentProgressing = "Progressing"
)

// ObjectDeploymentPhase specifies a phase that a deployment is in.
type ObjectDeploymentPhase string

// Well-known ObjectDeployment Phases for printing a Status in kubectl,
// see deprecation notice in ObjectDeploymentStatus for details.
const (
	ObjectDeploymentPhasePending     ObjectDeploymentPhase = "Pending"
	ObjectDeploymentPhaseAvailable   ObjectDeploymentPhase = "Available"
	ObjectDeploymentPhaseNotReady    ObjectDeploymentPhase = "NotReady"
	ObjectDeploymentPhaseProgressing ObjectDeploymentPhase = "Progressing"
)

// ObjectDeployment is the Schema for the ObjectDeployments API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={"objdeploy","od"}
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ObjectDeploymentSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ObjectDeploymentStatus `json:"status,omitempty"`
}

// ObjectDeploymentList contains a list of ObjectDeployments
// +kubebuilder:object:root=true
type ObjectDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectDeployment `json:"items"`
}

func init() { register(&ObjectDeployment{}, &ObjectDeploymentList{}) }
