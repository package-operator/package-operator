package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// HostedClusterPackage defines package to be rolled out on every HyperShift HostedCluster.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=hcpkg
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type HostedClusterPackage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HostedClusterPackageSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase: Pending}
	Status HostedClusterPackageStatus `json:"status,omitempty"`
}

// HostedClusterPackageSpec is the description of a HostedClusterPackage.
type HostedClusterPackageSpec struct {
	Strategy              HostedClusterPackageStrategy `json:"strategy"`
	HostedClusterSelector metav1.LabelSelector         `json:"hostedClusterSelector"`
	Template              PackageTemplateSpec          `json:"template"`
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
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase                            HostedClusterPackageStatusPhase `json:"phase,omitempty"`
	HostedClusterPackageCountsStatus `json:",inline"`
	// Count of packages found by partition.
	Partitions []HostedClusterPackagePartitionStatus `json:"partitions,omitempty"`
	// Processing set of packages during upgrade.
	Processing []HostedClusterPackageRefStatus `json:"processing,omitempty"`
}

// HostedClusterPackageStatusPhase is a human-readable status of the HostedClusterPackage.
type HostedClusterPackageStatusPhase string

// Well-known HostedClusterPackage Phases for printing a Status in kubectl,
// see deprecation notice in HostedClusterPackageStatus for details.
const (
	HostedClusterPackagePhasePending     HostedClusterPackageStatusPhase = "Pending"
	HostedClusterPackagePhaseProgressing HostedClusterPackageStatusPhase = "Progressing"
	HostedClusterPackagePhaseCompleted   HostedClusterPackageStatusPhase = "Completed"
)

// HostedClusterPackagePartitionStatus describes the status of a partition.
type HostedClusterPackagePartitionStatus struct {
	// Name of the partition.
	Name                             string `json:"name"`
	HostedClusterPackageCountsStatus `json:",inline"`
}

// HostedClusterPackageCountsStatus counts the status of Packages.
type HostedClusterPackageCountsStatus struct {
	// +optional
	Total int32 `json:"total,omitempty"`
	// +optional
	Found int32 `json:"found,omitempty"`
	// +optional
	Available int32 `json:"available,omitempty"`
	// +optional
	Updated int32 `json:"updated,omitempty"`
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
