package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Specifies the lifecycle state of the ObjectSet.
type ObjectSetLifecycleState string

const (
	// "Active" is the default lifecycle state.
	ObjectSetLifecycleStateActive ObjectSetLifecycleState = "Active"
	// "Paused" disables reconciliation of the ObjectSet.
	// Only Status updates will still propagated, but object changes will not be reconciled.
	ObjectSetLifecycleStatePaused ObjectSetLifecycleState = "Paused"
	// "Archived" disables reconciliation while also "scaling to zero",
	// which deletes all objects that are not excluded via the pausedFor property and
	// removes itself from the owner list of all other objects previously under management.
	ObjectSetLifecycleStateArchived ObjectSetLifecycleState = "Archived"
)

// ObjectSet specification.
type ObjectSetTemplateSpec struct {
	// Reconcile phase configuration for a ObjectSet.
	// Phases will be reconciled in order and the contained objects checked
	// against given probes before continuing with the next phase.
	Phases []ObjectSetTemplatePhase `json:"phases"`
	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	AvailabilityProbes []ObjectSetProbe `json:"availabilityProbes"`
}

// ObjectSet reconcile phase.
type ObjectSetTemplatePhase struct {
	// Name of the reconcile phase, must be unique within a ObjectSet.
	Name string `json:"name"`
	// Objects belonging to this phase.
	Objects []ObjectSetObject `json:"objects"`
}

// An object that is part of the phase of an ObjectSet.
type ObjectSetObject struct {
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	// +example={apiVersion: apps/v1, kind: Deployment, metadata: {name: example-deployment}}
	Object runtime.RawExtension `json:"object"`
}

// ObjectSetStatus defines the observed state of a ObjectSet.
type ObjectSetStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Deprecated: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase ObjectSetStatusPhase `json:"phase,omitempty"`
	// List of objects, the controller has paused reconciliation on.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
}

// Specifies that the reconciliation of a specific object should be paused.
type ObjectSetPausedObject struct {
	// Object Kind.
	// +example=Deployment
	Kind string `json:"kind"`
	// Object Group.
	// +example=apps
	Group string `json:"group"`
	// Object Name.
	// +example=example-deployment
	Name string `json:"name"`
}

// ObjectSet Condition Types.
const (
	// Available indicates that all objects pass their availability probe.
	ObjectSetAvailable = "Available"
	// Paused indicates that object changes are no reconciled, but status is still reported.
	ObjectSetPaused = "Paused"
	// Archived indicates that the ObjectSet is "scaled to zero"
	// meaning that all objects under management are cleaned up and status is no longer reported.
	// The ObjectSet might be stay on the cluster as revision tombstone to facilitate roll back.
	ObjectSetArchived = "Archived"
	// Succeeded condition is only set once,
	// after a ObjectSet became Available for the first time.
	ObjectSetSucceeded = "Succeeded"
)

type ObjectSetStatusPhase string

// Well-known ObjectSet Phases for printing a Status in kubectl,
// see deprecation notice in ObjectSetStatus for details.
const (
	// Default phase, when object is created and has not been serviced by an controller.
	ObjectSetStatusPhasePending ObjectSetStatusPhase = "Pending"
	// Available maps to Available condition == True, if not overridden by a more specific status below.
	ObjectSetStatusPhaseAvailable ObjectSetStatusPhase = "Available"
	// NotReady maps to Available condition == False, if not overridden by a more specific status below.
	ObjectSetStatusPhaseNotReady ObjectSetStatusPhase = "NotReady"
	// Paused maps to the Paused condition.
	ObjectSetStatusPhasePaused ObjectSetStatusPhase = "Paused"
	// Deprecated is reported, when only a subset of object is paused.
	ObjectSetStatusPhaseDeprecated ObjectSetStatusPhase = "Deprecated"
	// Archived maps to the Archived condition.
	ObjectSetStatusPhaseArchived ObjectSetStatusPhase = "Archived"
)

// ObjectSetProbe define how ObjectSets check their children for their status.
type ObjectSetProbe struct {
	// Probe configuration parameters.
	Probes []Probe `json:"probes"`
	// Selector specifies which objects this probe should target.
	Selector ProbeSelector `json:"selector"`
}

type ProbeSelectorType string

const (
	ProbeSelectorKind ProbeSelectorType = "Kind"
)

type ProbeSelector struct {
	// Kind specific configuration parameters. Only present if Type = Kind.
	Kind *PackageProbeKindSpec `json:"kind,omitempty"`
}

// Kind package probe parameters.
type PackageProbeKindSpec struct {
	// Object Group to apply a probe to.
	// +example=apps
	Group string `json:"group"`
	// Object Kind to apply a probe to.
	// +example=Deployment
	Kind string `json:"kind"`
}

// Defines probe parameters to check parts of a package.
type Probe struct {
	Condition   *ProbeConditionSpec   `json:"condition,omitempty"`
	FieldsEqual *ProbeFieldsEqualSpec `json:"fieldsEqual,omitempty"`
}

// Condition Probe parameters.
type ProbeConditionSpec struct {
	// Condition type to probe for.
	// +example=Available
	Type string `json:"type"`
	// Condition status to probe for.
	// +kubebuilder:default="True"
	Status string `json:"status"`
}

// Compares two fields specified by JSON Paths.
type ProbeFieldsEqualSpec struct {
	// +example=.spec.fieldA
	FieldA string `json:"fieldA"`
	// +example=.status.fieldB
	FieldB string `json:"fieldB"`
}
