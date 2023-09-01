//go:build mage
// +build mage

package main

// Must panics if the given error is not nil.
func must(err error) {
	if err != nil {
		panic(err)
	}
}
