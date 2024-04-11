//go:build integration

package packageoperator

import (
	"testing"
)

// Simple Setup, Pause and Teardown test.
func TestObjectSet_setupPauseTeardown(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetSetupPauseTeardownTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_handover(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetHandoverTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_orphanCascadeDeletion(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetOrphanCascadeDeletionTest(t, "default", test.class)
		})
	}
}
