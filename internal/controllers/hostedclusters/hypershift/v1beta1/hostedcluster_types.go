// The package v1beta1 contains some API Schema definitions for the v1beta1 version of some Hypershift API group.
// https://github.com/openshift/hypershift does not put its API definitions in a submodule, so we took what we needed
// from
// https://github.com/openshift/hypershift/blob/ab40031a6e551e5c9e674c4cfaf524f566898fce/api/hypershift/v1beta1/hostedcluster_types.go
// to avoid having to
// import all of Hypershift.
// +kubebuilder:object:generate=true
// +groupName=hypershift.openshift.io
package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "hypershift.openshift.io", Version: "v1beta1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&HostedCluster{}, &HostedClusterList{})
}

// HostedClusterSpec is the desired behavior of a HostedCluster.
type HostedClusterSpec struct {
	// NodeSelector when specified, is propagated to all control plane Deployments
	// and Stateful sets running management side.
	// It must be satisfied by the management Nodes for the pods to be scheduled.
	// Otherwise the HostedCluster will enter a degraded state.
	// Changes to this field will propagate to existing Deployments and StatefulSets.
	// +kubebuilder:validation:XValidation:rule="size(self) <= 20",message="nodeSelector map can have at most 20 entries"
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// More conditions at
// https://github.com/openshift/hypershift/blob/ab40031a6e551e5c9e674c4cfaf524f566898fce/api/hypershift/v1beta1/hostedcluster_conditions.go
// HostedCluster conditions.
const (
	// HostedClusterAvailable indicates whether the HostedCluster has a healthy
	// control plane.
	HostedClusterAvailable = "Available"
)

// HostedClusterStatus is the latest observed status of a HostedCluster.
type HostedClusterStatus struct {
	// KubeConfig is a reference to the secret containing the default kubeconfig
	// for the cluster.
	// +optional
	KubeConfig *corev1.LocalObjectReference `json:"kubeconfig,omitempty"`

	// KubeadminPassword is a reference to the secret that contains the initial
	// kubeadmin user password for the guest cluster.
	// +optional
	KubeadminPassword *corev1.LocalObjectReference `json:"kubeadminPassword,omitempty"`

	// Conditions represents the latest available observations of a control
	// plane's current state.
	Conditions []metav1.Condition `json:"conditions"`
}

// +genclient

// HostedCluster is the primary representation of a HyperShift cluster and encapsulates
// the control plane and common data plane configuration. Creating a HostedCluster
// results in a fully functional OpenShift control plane with no attached nodes.
// To support workloads (e.g. pods), a HostedCluster may have one or more associated
// NodePool resources.
//
// +kubebuilder:object:root=true
type HostedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// HostedClusterSpec is the desired behavior of a HostedCluster.
	Spec HostedClusterSpec `json:"spec,omitempty"`

	// Status is the latest observed status of the HostedCluster.
	Status HostedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// HostedClusterList contains a list of HostedCluster.
type HostedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostedCluster `json:"items"`
}
