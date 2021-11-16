package controllers

import "errors"

var (
	// This error is returned when a reconciled child object already
	// exists and is not owned by the current controller/addon
	ErrNotOwnedByUs = errors.New("object is not owned by us")
)
