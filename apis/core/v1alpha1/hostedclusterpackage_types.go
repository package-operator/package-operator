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
// +kubebuilder:printcolumn:name="Generation",type=string,JSONPath=`.status.observedGeneration`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type HostedClusterPackage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostedClusterPackageSpec   `json:"spec,omitempty"`
	Status HostedClusterPackageStatus `json:"status,omitempty"`
}

// HostedClusterPackageSpec is the description of a HostedClusterPackage.
type HostedClusterPackageSpec struct {
	Strategy HostedClusterPackageStrategy `json:"strategy"`
	// HostedClusterSelector is a label query matching HostedClusters that the Package should be rolled out to.
	HostedClusterSelector metav1.LabelSelector `json:"hostedClusterSelector,omitempty"`
	// PackageSpec describes the Package that should be created when new
	// HostedClusters matching the hostedClusterSelector are detected.
	PackageSpec PackageSpec `json:"spec"`
}

// HostedClusterPackageStrategy describes the rollout strategy for a HostedClusterPackage.
type HostedClusterPackageStrategy struct {
	// Updates all matching Packages instantly and all at the same time.
	Instant *HostedClusterPackageStrategyInstant `json:"instant,omitempty"`
	// Performs a rolling upgrade according to maxUnavailable and partition settings.
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
	// Partition HostedClusters by label value.
	// All packages in the same partition will have to be upgraded
	// before progressing to the next partition.
	Partition *HostedClusterPackagePartitionSpec `json:"partition,omitempty"`
}

// HostedClusterPackagePartitionSpec describes settings to partition HostedClusters into groups for upgrades.
// Partitions will be upgraded after each other:
// Upgrades in the next partition will only start after the previous has finished.
type HostedClusterPackagePartitionSpec struct {
	// LabelKey defines a labelKey to group objects on.
	// +kubebuilder:validation:MinLength=1
	LabelKey string `json:"labelKey"`
	// Controls how partitions are ordered.
	// By default items will be sorted AlphaNumeric ascending.
	Order *HostedClusterPackagePartitionOrderSpec `json:"order,omitempty"`
}

// HostedClusterPackagePartitionOrderSpec describes ordering for a partition.
type HostedClusterPackagePartitionOrderSpec struct {
	// Allows to define a static partition order.
	// The special * key matches anything not explicitly part of the list.
	Static []string `json:"static,omitempty"`
}

// HostedClusterPackageStatus describes the status of a HostedClusterPackage.
type HostedClusterPackageStatus struct {
	// Conditions is a list of status conditions ths object is in.
	// +example=[{type: "Available", status: "True", reason: "Available",  message: "Latest Revision is Available."}]
	Conditions                       []metav1.Condition `json:"conditions,omitempty"`
	HostedClusterPackageCountsStatus `json:",inline"`
	// Count of packages found by partition.
	Partitions []HostedClusterPackagePartitionStatus `json:"partitions,omitempty"`
	// Processing set of packages during upgrade.
	Processing []HostedClusterPackageRefStatus `json:"processing,omitempty"`
}

// HostedClusterPackagePartitionStatus describes the status of a partition.
type HostedClusterPackagePartitionStatus struct {
	// Name of the partition.
	Name                             string `json:"name"`
	HostedClusterPackageCountsStatus `json:",inline"`
}

// HostedClusterPackageCountsStatus counts the status of Packages.
type HostedClusterPackageCountsStatus struct {
	// +optional
	ObservedGeneration int32 `json:"observedGeneration,omitempty"`
	// +optional
	AvailablePackages int32 `json:"availablePackages,omitempty"`
	// +optional
	ReadyPackages int32 `json:"readyPackages,omitempty"`
	// +optional
	UnavailablePackages int32 `json:"unavailablePackages,omitempty"`
	// +optional
	UpdatedPackages int32 `json:"updatedPackages,omitempty"`
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
	Items           []Package `json:"items"`
}

func init() { register(&HostedClusterPackage{}, &HostedClusterPackageList{}) }
