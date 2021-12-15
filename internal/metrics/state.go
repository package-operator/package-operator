package metrics

import (
	"sync"
)

// addonState is a helper type that will keep track of an addon and its Status.
// This will used for updating metrics
type addonState struct {
	conditionMapping map[string]addonCondition
	mux              sync.RWMutex
}

// used by addonState for storing if an addon is available and/or paused
type addonCondition struct {
	available bool
	paused    bool
}

var stateObj *addonState

func newAddonState() *addonState {
	return &addonState{
		conditionMapping: map[string]addonCondition{},
	}
}

func init() {

	// Singleton for initializing addonStateInstance
	if stateObj == nil {
		stateObj = newAddonState()
	}
}
