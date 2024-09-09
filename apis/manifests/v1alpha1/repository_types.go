package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Repository is the k8s resource that represents a package repository.
// +kubebuilder:object:root=true
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func init() { register(&Repository{}) }
