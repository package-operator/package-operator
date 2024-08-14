package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterObjectDeploymentSpec defines the desired state of a ClusterObjectDeployment.
type ClusterObjectDeploymentSpec struct {
	// Number of old revisions in the form of archived ObjectSets to keep.
	// +kubebuilder:default=10
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
	// Selector targets ObjectSets managed by this Deployment.
	Selector metav1.LabelSelector `json:"selector"`
	// Template to create new ObjectSets from.
	Template ObjectSetTemplate `json:"template"`
}

// ClusterObjectDeploymentStatus defines the observed state of a ClusterObjectDeployment.
type ClusterObjectDeploymentStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase ObjectDeploymentPhase `json:"phase,omitempty"`
	// Count of hash collisions of the ClusterObjectDeployment.
	CollisionCount *int32 `json:"collisionCount,omitempty"`
	// Computed TemplateHash.
	TemplateHash string `json:"templateHash,omitempty"`
	// Deployment revision.
	Revision int64 `json:"revision,omitempty"`
	// References (Cluster)ObjectSets controlled by this instance.
	ControllerOf []ControlledObjectReference `json:"controllerOf,omitempty"`
}

// ClusterObjectDeployment is the Schema for the ClusterObjectDeployments API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName={"clobjdeploy","cod"}
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterObjectDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterObjectDeploymentSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ClusterObjectDeploymentStatus `json:"status,omitempty"`
}

// ClusterObjectDeploymentList contains a list of ClusterObjectDeployments
// +kubebuilder:object:root=true
type ClusterObjectDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterObjectDeployment `json:"items"`
}

func init() { register(&ClusterObjectDeployment{}, &ClusterObjectDeploymentList{}) }
