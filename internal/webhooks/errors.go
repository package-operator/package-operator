package webhooks

import "errors"

var (
	errObjectSetTemplatePhaseImmutable = errors.New("ObjectSetTemplatePhase is immutable")
	errObjectSetTemplateSpecImmutable  = errors.New("ObjectSetTemplateSpec is immutable")
	errPreviousImmutable               = errors.New(".spec.Previous is immutable")
	errRevisionImmutable               = errors.New(".spec.Revision is immutable")
	errAvailabilityProbesImmutable     = errors.New(".spec.AvailabilityProbes is immutable")
)
