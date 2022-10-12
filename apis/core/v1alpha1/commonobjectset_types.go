package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
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
	// Name of the reconcile phase. Must be unique within a ObjectSet.
	Name string `json:"name"`
	// If non empty, the ObjectSet controller will delegate phase reconciliation to another controller, by creating an ObjectSetPhase object.
	// If set to the string "default" the built-in Package Operator ObjectSetPhase controller will reconcile the object in the same way the ObjectSet would.
	// If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects.
	Class string `json:"class,omitempty"`
	// Objects belonging to this phase.
	Objects []ObjectSetObject `json:"objects"`
}

// An object that is part of the phase of an ObjectSet.
type ObjectSetObject struct {
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	// +example={apiVersion: apps/v1, kind: Deployment, metadata: {name: example-deployment}}
	Object unstructured.Unstructured `json:"object"`
}

// ObjectSet Condition Types.
const (
	// Available indicates that all objects pass their availability probe.
	ObjectSetAvailable = "Available"
	// Paused indicates that object changes are not reconciled, but status is still reported.
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

// Selects a subset of objects to apply probes to.
// e.g. ensures that probes defined for apps/Deployments are not checked against ConfigMaps.
type ProbeSelector struct {
	// Kind and API Group of the object to probe.
	Kind *PackageProbeKindSpec `json:"kind"`
	// Further sub-selects objects based on a Label Selector.
	// +example={matchLabels: {app.kubernetes.io/name: example-operator}}
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// Kind package probe parameters.
// selects objects based on Kind and API Group.
type PackageProbeKindSpec struct {
	// Object Group to apply a probe to.
	// +example=apps
	Group string `json:"group"`
	// Object Kind to apply a probe to.
	// +example=Deployment
	Kind string `json:"kind"`
}

// Defines probe parameters. Only one can be filled.
type Probe struct {
	Condition   *ProbeConditionSpec   `json:"condition,omitempty"`
	FieldsEqual *ProbeFieldsEqualSpec `json:"fieldsEqual,omitempty"`
}

// Checks whether or not the object reports a condition with given type and status.
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
	// First field for comparison.
	// +example=.spec.fieldA
	FieldA string `json:"fieldA"`
	// Second field for comparison.
	// +example=.status.fieldB
	FieldB string `json:"fieldB"`
}

// References a previous revision of an ObjectSet or ClusterObjectSet.
type PreviousRevisionReference struct {
	// Name of a previous revision.
	// +example=previous-revision
	Name string `json:"name"`
}

// References remote phases aka ObjectSetPhase/ClusterObjectSetPhase objects to which a phase is delegated.
type RemotePhaseReference struct {
	Name string    `json:"name"`
	UID  types.UID `json:"uid"`
}

// References an object controlled by this ObjectSet/ObjectSetPhase.
type ControlledObjectReference struct {
	// Object Kind.
	Kind string `json:"kind"`
	// Object Group.
	Group string `json:"group"`
	// Object Name.
	Name string `json:"name"`
	// Object Namespace.
	Namespace string `json:"namespace,omitempty"`
}
