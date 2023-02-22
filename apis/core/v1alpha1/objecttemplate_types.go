package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectTemplates
// +kubebuilder:object:root=true
type ObjectTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ObjectTemplateSpec `json:"spec,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ObjectTemplate{})
}
