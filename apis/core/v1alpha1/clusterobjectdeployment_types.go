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
	// If Paused is true, the object and its children will not be reconciled.
	Paused bool `json:"paused,omitempty"`
}

// ClusterObjectDeploymentStatus defines the observed state of a ClusterObjectDeployment.
type ClusterObjectDeploymentStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Count of hash collisions of the ClusterObjectDeployment.
	CollisionCount *int32 `json:"collisionCount,omitempty"`
	// Computed TemplateHash.
	TemplateHash string `json:"templateHash,omitempty"`
	// Deployment revision.
	Revision int64 `json:"revision,omitempty"`
	// ControllerOf references the owned ClusterObjectSet revisions.
	ControllerOf []ControlledObjectReference `json:"controllerOf,omitempty"`
}

// ClusterObjectDeployment is the Schema for the ClusterObjectDeployments API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName={"clobjdeploy","cod"}
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.revision`
// +kubebuilder:printcolumn:name="Progressing",type=string,JSONPath=`.status.conditions[?(@.type=="Progressing")].status`
// +kubebuilder:printcolumn:name="Paused",type=string,JSONPath=`.status.conditions[?(@.type=="Paused")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterObjectDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterObjectDeploymentSpec   `json:"spec,omitempty"`
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
