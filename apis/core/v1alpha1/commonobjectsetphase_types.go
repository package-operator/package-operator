package v1alpha1

// ObjectSetPhase Condition Types.
const (
	// Available indicates that all objects pass their availability probe.
	ObjectSetPhaseAvailable = "Available"
	// Paused indicates that object changes are not reconciled, but status is still reported.
	ObjectSetPhasePaused = "Paused"
)

// ObjectSetPhaseClassLabel is the label key for the phase class.
const ObjectSetPhaseClassLabel = "package-operator.run/phase-class"
