package controllers

import (
	"time"
)

type WithInitialBackoff time.Duration

func (w WithInitialBackoff) ConfigureBackoff(c *BackoffConfig) {
	val := time.Duration(w)

	c.InitialBackoff = &val
}

type WithMaxBackoff time.Duration

func (w WithMaxBackoff) ConfigureBackoff(c *BackoffConfig) {
	val := time.Duration(w)

	c.MaxBackoff = &val
}
