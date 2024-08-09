package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretSync synchronizes a singlular source object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName={"objsync","osync","os"}
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type SecretSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec SecretSyncSpec `json:"spec"`

	// +kubebuilder:default={phase:Pending}
	Status SecretSyncStatus `json:"status,omitempty"`
}

// SecretSyncList contains a list of SecretSyncs.
// +kubebuilder:object:root=true
type SecretSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretSync `json:"items"`
}

// SecretSyncSpec contains the desired config if an SecretSync.
type SecretSyncSpec struct {
	// Disables reconciliation of the SecretSync.
	// Only Status updates will still be reported, but object changes will not be reconciled.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// +kubebuilder:default={watch:{}}
	Strategy SecretSyncStrategy `json:"strategy"`

	// +kubebuilder:validation:Required
	Src NamespacedName `json:"src"`

	// +kubebuilder:validation:Required
	// +kubebuilder:MaxItems=32
	// +kubebuilder:validation:UniqueItems=true
	Dest []NamespacedName `json:"dest"`
}

// SecretSyncStatus contains the observed state of a SecretSync.
type SecretSyncStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Phase is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase SecretSyncStatusPhase `json:"phase,omitempty"`
	// TODO: maybe include last sync timestamp
	// References all objects controlled by this instance.
	ControllerOf []NamespacedName `json:"controllerOf,omitempty"`
}

// SecretSyncStrategy configures which strategy is used for synchronization. Exactly one strategy must be configured at any given time.
// Defaults to the `Watch` strategy if not specified.
// +kubebuilder:validation:XValidation:rule="self.exists_one(_, true)", message="exactly one strategy is must be configured"
//
//nolint:lll
type SecretSyncStrategy struct {
	// The `Poll` strategy synchronizes source and destinations in regular intervals which can be configured.
	Poll *SecretSyncStrategyPoll `json:"poll,omitempty"`

	// The `Watch` strategy watches the source object for changes and queues re-synchronization whenever a the manager observes a write to a source.
	// Caution: package-operator will add a label to the source object to make it visible to its in-memory caches which can lead to a write cascade on the object
	// if it is managed by a controller that insists on owning the full shape of the object. You can use the `Poll` strategy instead if you observe this happening,
	// and have reasons not to change the behaviour of the controller in question.
	Watch *SecretSyncStrategyWatch `json:"watch,omitempty"`
}

// SecretSyncStrategyPoll contains configuration for the `Poll` sync strategy.
type SecretSyncStrategyPoll struct {
	// Specifies the poll interval as a string which can be parsed to a time.Duration
	// by [time.ParseDuration](https://pkg.go.dev/time#ParseDuration).
	// +kubebuilder:validation:Required
	Interval metav1.Duration `json:"interval"`
}

// SecretSyncStrategyWatch contains configuration for the `Watch` sync strategy.
type SecretSyncStrategyWatch struct{}

// NamespacedName container.
type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// SecretSyncStatusPhase defines the status phase of a SecretSync object.
type SecretSyncStatusPhase string

// Well-known SecretSync Phases for printing a Status in kubectl,
// see deprecation notice in SecretSyncStatus for details.
const (
	// Default phase, when object is created and has not been serviced by a controller yet.
	SecretSyncStatusPhasePending SecretSyncStatusPhase = "Pending"
	// Sync maps to Sync condition == True, if not overridden by a more specific status below.
	SecretSyncStatusPhaseSync SecretSyncStatusPhase = "Sync"
	// Paused maps to the Paused condition.
	SecretSyncStatusPhasePaused SecretSyncStatusPhase = "Paused"
)

// func init() { register(&SecretSync{}, &ClusterObjectSliceList{}) }
