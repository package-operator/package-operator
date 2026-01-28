package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectTemplate contain a go template of a Kubernetes manifest. This manifest is then templated with the
// sources provided in the .Spec.Sources. The sources can only come from objects within the same namespace
// as the ObjectTemplate.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName={"objtmpl","ot"}
// +kubebuilder:printcolumn:name="Invalid",type=string,JSONPath=`.status.conditions[?(@.type=="package-operator.run/Invalid")].status`
type ObjectTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectTemplateSpec   `json:"spec,omitempty"`
	Status ObjectTemplateStatus `json:"status,omitempty"`
}

// ObjectTemplateList contains a list of ObjectTemplates.
// +kubebuilder:object:root=true
type ObjectTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectTemplate `json:"items"`
}

func init() { register(&ObjectTemplate{}, &ObjectTemplateList{}) }
