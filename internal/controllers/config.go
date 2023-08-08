package controllers

import (
	"time"

	"k8s.io/client-go/util/flowcontrol"
)

const (
	// Use this delay if you want to retry an invalid user configuration.
	// e.g.:
	// - referencing objects that don't exist.
	// - object collisions.
	// - missing permissions.
	DefaultGlobalMissConfigurationRetry = 30 * time.Second

	DefaultInitialBackoff = 10 * time.Second
	DefaultMaxBackoff     = 300 * time.Second
)

type BackoffConfig struct {
	InitialBackoff *time.Duration
	MaxBackoff     *time.Duration
}

func (c *BackoffConfig) Option(opts ...BackoffOption) {
	for _, opt := range opts {
		opt.ConfigureBackoff(c)
	}
}

type BackoffOption interface {
	ConfigureBackoff(*BackoffConfig)
}

func (c *BackoffConfig) Default() {
	var (
		initialBackoff = DefaultInitialBackoff
		maxBackoff     = DefaultMaxBackoff
	)

	if c.InitialBackoff == nil {
		c.InitialBackoff = &initialBackoff
	}
	if c.MaxBackoff == nil {
		c.MaxBackoff = &maxBackoff
	}
}

func (c *BackoffConfig) GetBackoff() *flowcontrol.Backoff {
	return flowcontrol.NewBackOff(*c.InitialBackoff, *c.MaxBackoff)
}
