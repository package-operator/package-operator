package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// ObjectSetRevisionAnnotation annotations holds a revision generation number to order ObjectSets.
const ObjectSetRevisionAnnotation = "package-operator.run/revision"

// ObjectSetLifecycleState specifies the lifecycle state of the ObjectSet.
type ObjectSetLifecycleState string

const (
	// ObjectSetLifecycleStateActive / "Active" is the default lifecycle state.
	ObjectSetLifecycleStateActive ObjectSetLifecycleState = "Active"
	// ObjectSetLifecycleStatePaused / "Paused" disables reconciliation of the ObjectSet.
	// Only Status updates will still propagated, but object changes will not be reconciled.
	ObjectSetLifecycleStatePaused ObjectSetLifecycleState = "Paused"
	// ObjectSetLifecycleStateArchived / "Archived" disables reconciliation while also "scaling to zero",
	// which deletes all objects that are not excluded via the pausedFor property and
	// removes itself from the owner list of all other objects previously under management.
	ObjectSetLifecycleStateArchived ObjectSetLifecycleState = "Archived"
)

// ObjectSetTemplateSpec defines an object set.
// WARNING: when modifying fields in ObjectSetTemplateSpec
// also update validation rules in (Cluster)ObjectSetSpec.
type ObjectSetTemplateSpec struct {
	// Reconcile phase configuration for a ObjectSet.
	// Phases will be reconciled in order and the contained objects checked
	// against given probes before continuing with the next phase.
	Phases []ObjectSetTemplatePhase `json:"phases,omitempty"`
	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	AvailabilityProbes []ObjectSetProbe `json:"availabilityProbes,omitempty"`
	// Success Delay Seconds applies a wait period from the time an
	// Object Set is available to the time it is marked as successful.
	// This can be used to prevent false reporting of success when
	// the underlying objects may initially satisfy the availability
	// probes, but are ultimately unstable.
	SuccessDelaySeconds int32 `json:"successDelaySeconds,omitempty"`
}

// ObjectSetTemplatePhase configures the reconcile phase of ObjectSets.
type ObjectSetTemplatePhase struct {
	// Name of the reconcile phase. Must be unique within a ObjectSet.
	Name string `json:"name"`
	// If non empty, the ObjectSet controller will delegate phase reconciliation
	// to another controller, by creating an ObjectSetPhase object. If set to the
	// string "default" the built-in Package Operator ObjectSetPhase controller
	// will reconcile the object in the same way the ObjectSet would. If set to
	// any other string, an out-of-tree controller needs to be present to handle
	// ObjectSetPhase objects.
	Class string `json:"class,omitempty"`
	// Objects belonging to this phase.
	Objects []ObjectSetObject `json:"objects,omitempty"`

	// References to ObjectSlices containing objects for this phase.
	Slices []string `json:"slices,omitempty"`
}

// ObjectSetObject is an object that is part of the phase of an ObjectSet.
type ObjectSetObject struct {
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	// +example={apiVersion: apps/v1, kind: Deployment, metadata: {name: example-deployment}}
	Object unstructured.Unstructured `json:"object"`
	// Collision protection prevents Package Operator from working on objects already under
	// management by a different operator.
	// +kubebuilder:default=Prevent
	CollisionProtection CollisionProtection `json:"collisionProtection,omitempty"`
	// Maps conditions from this object into the Package Operator APIs.
	ConditionMappings []ConditionMapping `json:"conditionMappings,omitempty"`
}

func (o ObjectSetObject) String() string {
	obj := o.Object

	return fmt.Sprintf("object %s/%s kind:%s", obj.GetNamespace(), obj.GetName(), obj.GetKind())
}

// CollisionProtection specifies if and how PKO prevent ownership collisions.
type CollisionProtection string

const (
	// CollisionProtectionPrevent prevents owner collisions entirely by only allowing
	// Package Operator to work with objects itself has created.
	CollisionProtectionPrevent CollisionProtection = "Prevent"
	// CollisionProtectionIfNoController allows Package Operator to patch and override
	// objects already present if they are not owned by another controller.
	CollisionProtectionIfNoController CollisionProtection = "IfNoController"
	// CollisionProtectionNone allows Package Operator to patch and override objects
	// already present and owned by other controllers.
	// Be careful! This setting may cause multiple controllers to fight over a resource,
	// causing load on the API server and etcd.
	CollisionProtectionNone CollisionProtection = "None"
)

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
	// InTransition condition is True when the ObjectSet is not in control of all objects defined in spec.
	// This holds true during rollout of the first instance or while handing over objects between two ObjectSets.
	ObjectSetInTransition = "InTransition"
)

// ObjectSetProbe define how ObjectSets check their children for their status.
type ObjectSetProbe struct {
	// Probe configuration parameters.
	Probes []Probe `json:"probes"`
	// Selector specifies which objects this probe should target.
	Selector ProbeSelector `json:"selector"`
}

// ConditionMapping maps one condition type to another.
type ConditionMapping struct {
	// Source condition type.
	SourceType string `json:"sourceType"`
	// Destination condition type to report into Package Operator APIs.
	//nolint:lll
	// +kubebuilder:validation:Pattern=`[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]`
	DestinationType string `json:"destinationType"`
}

// ProbeSelector selects a subset of objects to apply probes to.
// e.g. ensures that probes defined for apps/Deployments are not checked against ConfigMaps.
type ProbeSelector struct {
	// Kind and API Group of the object to probe.
	Kind *PackageProbeKindSpec `json:"kind"`
	// Further sub-selects objects based on a Label Selector.
	// +example={matchLabels: {app.kubernetes.io/name: example-operator}}
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// PackageProbeKindSpec package probe parameters.
// selects objects based on Kind and API Group.
type PackageProbeKindSpec struct {
	// Object Group to apply a probe to.
	// +example=apps
	Group string `json:"group"`
	// Object Kind to apply a probe to.
	// +example=Deployment
	Kind string `json:"kind"`
}

// Probe defines probe parameters. Only one can be filled.
type Probe struct {
	Condition   *ProbeConditionSpec   `json:"condition,omitempty"`
	FieldsEqual *ProbeFieldsEqualSpec `json:"fieldsEqual,omitempty"`
	CEL         *ProbeCELSpec         `json:"cel,omitempty"`
}

// ProbeConditionSpec checks whether or not the object reports a condition with given type and status.
type ProbeConditionSpec struct {
	// Condition type to probe for.
	// +example=Available
	Type string `json:"type"`
	// Condition status to probe for.
	// +kubebuilder:default="True"
	Status string `json:"status"`
}

// ProbeFieldsEqualSpec compares two fields specified by JSON Paths.
type ProbeFieldsEqualSpec struct {
	// First field for comparison.
	// +example=.spec.fieldA
	FieldA string `json:"fieldA"`
	// Second field for comparison.
	// +example=.status.fieldB
	FieldB string `json:"fieldB"`
}

// ProbeCELSpec uses Common Expression Language (CEL) to probe an object.
// CEL rules have to evaluate to a boolean to be valid.
// See:
// https://kubernetes.io/docs/reference/using-api/cel
// https://github.com/google/cel-go
type ProbeCELSpec struct {
	// CEL rule to evaluate.
	// +example=self.metadata.name == "Hans"
	Rule string `json:"rule"`
	// Error message to output if rule evaluates to false.
	// +example=Object must be named Hans
	Message string `json:"message"`
}

// PreviousRevisionReference references a previous revision of an ObjectSet or ClusterObjectSet.
type PreviousRevisionReference struct {
	// Name of a previous revision.
	// +example=previous-revision
	Name string `json:"name"`
}

// RemotePhaseReference remote phases aka ObjectSetPhase/ClusterObjectSetPhase objects to which a phase is delegated.
type RemotePhaseReference struct {
	Name string    `json:"name"`
	UID  types.UID `json:"uid"`
}

// ControlledObjectReference an object controlled by this object.
type ControlledObjectReference struct {
	// Object Kind.
	Kind string `json:"kind"`
	// Object Group.
	Group string `json:"group"`
	// Object Name.
	Name string `json:"name"`
	// Object Namespace.
	Namespace string `json:"namespace,omitempty"`
	// Object Version.
	Version string `json:"version"`
}
