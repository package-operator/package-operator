package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterObjectTemplate contain a go template of a Kubernetes manifest. The manifest is then templated with the
// sources provided in the .Spec.Sources. The sources can come from objects from any namespace or cluster scoped
// objects.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type ClusterObjectTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectTemplateSpec   `json:"spec,omitempty"`
	Status ObjectTemplateStatus `json:"status,omitempty"`
}

// ClusterObjectTemplateList contains a list of ClusterObjectTemplates.
// +kubebuilder:object:root=true
type ClusterObjectTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterObjectTemplate `json:"items"`
}

func init() { register(&ClusterObjectTemplate{}, &ClusterObjectTemplateList{}) }
