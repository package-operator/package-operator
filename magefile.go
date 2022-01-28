//go:build mage
// +build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Testing and Linting
// -------------------

type Test mg.Namespace

// Runs unittests.
func (Test) Unit() error {
	return sh.RunWithV(map[string]string{
		// needed to enable race detector -race
		"CGO_ENABLED": "1",
	}, "go", "test", "-cover", "-v", "-race", "./internal/...", "./cmd/...")
}
