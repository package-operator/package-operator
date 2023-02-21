package v1alpha1

type AdoptionStrategyType string

const (
	// Static will set a static label on every object not yet having label keys set.
	AdoptionStrategyStatic AdoptionStrategyType = "Static"
	// RoundRobin will apply given labels via a round robin strategy.
	AdoptionStrategyRoundRobin AdoptionStrategyType = "RoundRobin"
)

type AdoptionStrategyStaticSpec struct {
	// Labels to set on objects.
	Labels map[string]string `json:"labels"`
}

type AdoptionStrategyRoundRobinSpec struct {
	// Labels to set always, no matter the round robin choice.
	Always map[string]string `json:"always"`
	// Options for the round robin strategy to choose from.
	// Only a single label set of all the provided options will be applied.
	Options []map[string]string `json:"options"`
}

// TargetAPI specifies an API to use for operations.
type TargetAPI struct {
	Group string `json:"group"`
	//+kubebuilder:validation:MinLength=1
	Version string `json:"version"`
	//+kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`
}

type AdoptionRoundRobinStatus struct {
	// Last index chosen by the round robin algorithm.
	LastIndex int `json:"lastIndex"`
}

const (
	// The active condition is True as long as the adoption process is active.
	AdoptionActive = "Active"
)

type AdoptionPhase string

// Well-known Adoption Phases for printing a Status in kubectl,
// see deprecation notice in AdoptionStatus for details.
const (
	AdoptionPhasePending AdoptionPhase = "Pending"
	AdoptionPhaseActive  AdoptionPhase = "Active"
)
