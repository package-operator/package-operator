package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// HostedClusterPackage defines package to be rolled out on every HyperShift HostedCluster.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcpkg
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Progressing",type=string,JSONPath=`.status.conditions[?(@.type=="Progressing")].status`
// +kubebuilder:printcolumn:name="ObservedGeneration",type=string,JSONPath=`.status.observedGeneration`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type HostedClusterPackage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostedClusterPackageSpec   `json:"spec,omitempty"`
	Status HostedClusterPackageStatus `json:"status,omitempty"`
}

// HostedClusterPackageSpec is the description of a HostedClusterPackage.
type HostedClusterPackageSpec struct {
	// +kubebuilder:default={instant:{}}
	Strategy HostedClusterPackageStrategy `json:"strategy"`
	// HostedClusterSelector is a label query matching HostedClusters that the Package should be rolled out to.
	HostedClusterSelector metav1.LabelSelector `json:"hostedClusterSelector,omitempty"`
	// Template describes the Package that should be created when new
	// HostedClusters matching the hostedClusterSelector are detected.
	Template PackageTemplateSpec `json:"template"`
	// Partition HostedClusters by label value.
	// All packages in the same partition will have to be upgraded
	// before progressing to the next partition.
	Partition *HostedClusterPackagePartitionSpec `json:"partition,omitempty"`
}

// HostedClusterPackageStrategy describes the rollout strategy for a HostedClusterPackage.
type HostedClusterPackageStrategy struct {
	// Updates all matching Packages instantly.
	Instant *HostedClusterPackageStrategyInstant `json:"instant,omitempty"`
	// Performs a rolling upgrade according to maxUnavailable.
	RollingUpgrade *HostedClusterPackageStrategyRollingUpgrade `json:"rollingUpgrade,omitempty"`
}

// HostedClusterPackageStrategyInstant describes the instant
// rollout strategy for a HostedClusterPackages.
type HostedClusterPackageStrategyInstant struct{}

// HostedClusterPackageStrategyRollingUpgrade describes the
// rolling upgrade strategy for HostedClusterPackages.
type HostedClusterPackageStrategyRollingUpgrade struct {
	// MaxUnavailable defines how many Packages may become unavailable during upgrade at the same time.
	// Cannot be below 1, because we cannot surge to create more instances.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	MaxUnavailable int `json:"maxUnavailable"`
}

// HostedClusterPackagePartitionSpec describes settings to partition HostedClusters into groups for upgrades.
// Partitions will be upgraded after each other:
// Upgrades in the next partition will only start after the previous has finished.
// HostedClusters without the label will be put into an implicit "unknown" group
// and will get upgraded LAST.
type HostedClusterPackagePartitionSpec struct {
	// LabelKey defines a labelKey to group objects on.
	// +kubebuilder:validation:MinLength=1
	LabelKey string `json:"labelKey"`
	// Controls how partitions are ordered.
	// By default items will be sorted AlphaNumeric ascending.
	Order *HostedClusterPackagePartitionOrderSpec `json:"order,omitempty"`
}

// HostedClusterPackagePartitionOrderAlphanumericAsc describes the alphanumeric
// ascending partition ordering for HostedClusterPackages.
type HostedClusterPackagePartitionOrderAlphanumericAsc struct{}

// HostedClusterPackagePartitionOrderSpec describes ordering for a partition.
type HostedClusterPackagePartitionOrderSpec struct {
	// Allows to define a static partition order.
	// The special * key matches anything not explicitly part of the list.
	// Unknown risk-groups or HostedClusters without label
	// will be put into an implicit "unknown" group and
	// will get upgraded LAST.
	Static          []string                                           `json:"static,omitempty"`
	AlphanumericAsc *HostedClusterPackagePartitionOrderAlphanumericAsc `json:"alphanumericAsc,omitempty"`
}

// HostedClusterPackageStatus describes the status of a HostedClusterPackage.
type HostedClusterPackageStatus struct {
	// Conditions is a list of status conditions this object is in.
	// +example=[{type: "Available", status: "True", reason: "Available",  message: "Latest Revision is Available."}]
	Conditions                       []metav1.Condition `json:"conditions,omitempty"`
	HostedClusterPackageCountsStatus `json:",inline"`
	// Count of packages found by partition.
	Partitions []HostedClusterPackagePartitionStatus `json:"partitions,omitempty"`
	// Processing set of packages during upgrade.
	Processing []HostedClusterPackageRefStatus `json:"processing,omitempty"`
}

const (
	// HostedClusterPackageAvailable indicates that all or a given percentage of managed Packages
	// are reporting a positive Available status condition.
	HostedClusterPackageAvailable = "Available"
	// HostedClusterPackageProgressing indicates that A rollout is currently ongoing.
	// This means that not all managed Packages have been upgraded to the latest specified version and configuration.
	HostedClusterPackageProgressing = "Progressing"
)

// HostedClusterPackagePartitionStatus describes the status of a partition.
type HostedClusterPackagePartitionStatus struct {
	// Name of the partition.
	Name                             string `json:"name"`
	HostedClusterPackageCountsStatus `json:",inline"`
}

// HostedClusterPackageCountsStatus counts the status of Packages.
type HostedClusterPackageCountsStatus struct {
	// The generation observed by the HostedClusterPackage controller.
	// +optional
	ObservedGeneration int32 `json:"observedGeneration,omitempty"`
	// Total number of available Packages ready for at least minReadySeconds
	// targeted by this HostedClusterPackage.
	// +optional
	AvailablePackages int32 `json:"availablePackages,omitempty"`
	// Managed Packages with a Progressing=False Condition.
	// +optional
	ReadyPackages int32 `json:"readyPackages,omitempty"`
	// Total number of unavailable packages targeted by this HostedClusterPackage. This is the total number of
	// Packages that are still required for the HostedClusterPackage to have 100% available capacity.
	// They may be packages that exist but aren’t available yet, or packages that haven’t been created.
	// +optional
	UnavailablePackages int32 `json:"unavailablePackages,omitempty"`
	// Total number of non-terminated Packages targeted by this HostedClusterPackage that have the desired template spec.
	// +optional
	UpdatedPackages int32 `json:"updatedPackages,omitempty"`
	// Total number of non-terminated Packages targeted by this HostedClusterPackage.
	// +optional
	Packages int32 `json:"packages,omitempty"`
}

// HostedClusterPackageRefStatus holds a reference to upgrades in-flight.
type HostedClusterPackageRefStatus struct {
	UID       types.UID `json:"uid"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace,omitempty"`
}

// HostedClusterPackageList contains a list of HostedClusterPackage.
// +kubebuilder:object:root=true
type HostedClusterPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostedClusterPackage `json:"items"`
}

func init() { register(&HostedClusterPackage{}, &HostedClusterPackageList{}) }
