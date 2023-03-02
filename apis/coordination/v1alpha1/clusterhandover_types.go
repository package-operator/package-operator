package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterHandover struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterHandoverSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase: Pending}
	Status ClusterHandoverStatus `json:"status,omitempty"`
}

type ClusterHandoverSpec struct {
	// Strategy to use when handing over objects between operators.
	Strategy HandoverStrategy `json:"strategy"`
	// TargetAPI to use for handover.
	TargetAPI TargetAPI `json:"targetAPI"`
	// Probes to check selected objects for availability.
	AvailabilityProbes []corev1alpha1.Probe `json:"availabilityProbes"`
}

// HandoverStrategy defines the strategy to handover objects.
type HandoverStrategy struct {
	// Type of handover strategy. Can be "Relabel".
	// +kubebuilder:default=Relabel
	// +kubebuilder:validation:Enum={"Relabel"}
	Type HandoverStrategyType `json:"type"`

	// Relabel handover strategy configuration.
	// Only present when type=Relabel.
	Relabel *HandoverStrategyRelabelSpec `json:"relabel,omitempty"`
}

type HandoverStrategyType string

const (
	// Relabel will change a specified label object after object.
	HandoverStrategyRelabel HandoverStrategyType = "Relabel"
)

// Relabel handover strategy definition.
type HandoverStrategyRelabelSpec struct {
	// LabelKey defines the labelKey to change the value of.
	// +kubebuilder:validation:MinLength=1
	LabelKey string `json:"labelKey"`

	// InitialValue defines the initial value of the label to search for.
	// +kubebuilder:validation:MinLength=1
	InitialValue string `json:"initialValue"`

	// ToValue defines the desired value of the label after handover.
	// +kubebuilder:validation:MinLength=1
	ToValue string `json:"toValue"`

	// Status path to validate that the new operator has taken over the object.
	// Must point to a field in the object that mirrors the value of .labelKey as seen by the operator.
	ObservedLabelValuePath string `json:"observedLabelValuePath"`

	// MaxUnavailable defines how many objects may become unavailable due to the handover at the same time.
	// Cannot be below 1, because we cannot surge while relabling to create more instances.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	MaxUnavailable int `json:"maxUnavailable"`
}

// TargetAPI specifis an API to use for operations.
type TargetAPI struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

type PartitionSpec struct {
	// LabelKey defines a labelKey to group objects on.
	// +kubebuilder:validation:MinLength=1
	LabelKey string `json:"labelKey"`
}

type PartitionOrderingSpec struct {
	// Type of handover strategy. Can be Numeric,AlphaNumeric,Static.
	// +kubebuilder:default=Static
	// +kubebuilder:validation:Enum={"Static"}
	Type HandoverStrategyType `json:"type"`
	// Static list of partitions in order.
	// Every label value not listed explicitly,
	// will be appended to the end of the list in AlphaNumeric order.
	Static []string `json:"static,omitempty"`
}

// ClusterHandoverStatus defines the observed state of a Package.
type ClusterHandoverStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase HandoverStatusPhase `json:"phase,omitempty"`
	// Total count of objects found.
	Total HandoverCountsStatus `json:"total,omitempty"`
	// Count of objects found by partition.
	Partitions []HandoverPartitionStatus `json:"partitions,omitempty"`
	// Processing set of objects during handover.
	Processing []HandoverRefStatus `json:"processing,omitempty"`
}

// Handover condition types.
const (
	// Completed tracks whether all found objects have been handed over.
	// Handover objects may become completed multiple times over their lifecycle, if new objects are created.
	HandoverCompleted = "Completed"
)

type HandoverStatusPhase string

// Well-known Package Phases for printing a Status in kubectl,
// see deprecation notice in PackageStatus for details.
const (
	PackagePhasePending     HandoverStatusPhase = "Pending"
	PackagePhaseProgressing HandoverStatusPhase = "Progressing"
	PackagePhaseUnpacking   HandoverStatusPhase = "Completed"
)

type HandoverPartitionStatus struct {
	// Name of the partition this status belongs to.
	Name                 string `json:"name"`
	HandoverCountsStatus `json:",inline"`
}

type HandoverCountsStatus struct {
	// +optional
	Found int32 `json:"found"`
	// +optional
	Available int32 `json:"available"`
	// +optional
	Updated int32 `json:"updated"`
}

type HandoverRefStatus struct {
	UID       types.UID `json:"uid"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace,omitempty"`
}

// PackageList contains a list of Packages.
// +kubebuilder:object:root=true
type ClusterHandoverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterHandover `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterHandover{}, &ClusterHandoverList{})
}
