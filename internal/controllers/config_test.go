package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/util/flowcontrol"
)

func TestConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 30*time.Second, DefaultGlobalMissConfigurationRetry)
	assert.Equal(t, 10*time.Second, DefaultInitialBackoff)
	assert.Equal(t, 300*time.Second, DefaultMaxBackoff)
}

func TestBackoffConfig_Option(t *testing.T) {
	t.Parallel()

	cfg := &BackoffConfig{}
	initialBackoff := 5 * time.Second
	maxBackoff := 60 * time.Second

	cfg.Option(
		WithInitialBackoff(initialBackoff),
		WithMaxBackoff(maxBackoff),
	)

	assert.Equal(t, &initialBackoff, cfg.InitialBackoff)
	assert.Equal(t, &maxBackoff, cfg.MaxBackoff)
}

func TestBackoffConfig_Default(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      BackoffConfig
		expected BackoffConfig
	}{
		{
			name: "empty config gets defaults",
			cfg:  BackoffConfig{},
			expected: BackoffConfig{
				InitialBackoff: func() *time.Duration { d := DefaultInitialBackoff; return &d }(),
				MaxBackoff:     func() *time.Duration { d := DefaultMaxBackoff; return &d }(),
			},
		},
		{
			name: "config with initial backoff preserves it",
			cfg: BackoffConfig{
				InitialBackoff: func() *time.Duration { d := 5 * time.Second; return &d }(),
			},
			expected: BackoffConfig{
				InitialBackoff: func() *time.Duration { d := 5 * time.Second; return &d }(),
				MaxBackoff:     func() *time.Duration { d := DefaultMaxBackoff; return &d }(),
			},
		},
		{
			name: "config with max backoff preserves it",
			cfg: BackoffConfig{
				MaxBackoff: func() *time.Duration { d := 60 * time.Second; return &d }(),
			},
			expected: BackoffConfig{
				InitialBackoff: func() *time.Duration { d := DefaultInitialBackoff; return &d }(),
				MaxBackoff:     func() *time.Duration { d := 60 * time.Second; return &d }(),
			},
		},
		{
			name: "config with both values preserves them",
			cfg: BackoffConfig{
				InitialBackoff: func() *time.Duration { d := 5 * time.Second; return &d }(),
				MaxBackoff:     func() *time.Duration { d := 60 * time.Second; return &d }(),
			},
			expected: BackoffConfig{
				InitialBackoff: func() *time.Duration { d := 5 * time.Second; return &d }(),
				MaxBackoff:     func() *time.Duration { d := 60 * time.Second; return &d }(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := test.cfg
			cfg.Default()

			assert.Equal(t, *test.expected.InitialBackoff, *cfg.InitialBackoff)
			assert.Equal(t, *test.expected.MaxBackoff, *cfg.MaxBackoff)
		})
	}
}

func TestBackoffConfig_GetBackoff(t *testing.T) {
	t.Parallel()

	cfg := &BackoffConfig{}
	cfg.Default()

	backoff := cfg.GetBackoff()

	assert.NotNil(t, backoff)
	assert.IsType(t, &flowcontrol.Backoff{}, backoff)

	// Test that the backoff was created with the correct parameters
	// We can't directly inspect the flowcontrol.Backoff fields since they're private,
	// but we can verify it's created without error
	assert.NotNil(t, backoff)
}

func TestBackoffConfig_GetBackoff_CustomValues(t *testing.T) {
	t.Parallel()

	initialBackoff := 2 * time.Second
	maxBackoff := 30 * time.Second

	cfg := &BackoffConfig{
		InitialBackoff: &initialBackoff,
		MaxBackoff:     &maxBackoff,
	}

	backoff := cfg.GetBackoff()

	assert.NotNil(t, backoff)
	assert.IsType(t, &flowcontrol.Backoff{}, backoff)
}

func TestBackoffConfig_ChainedOperations(t *testing.T) {
	t.Parallel()

	cfg := &BackoffConfig{}
	cfg.Option(WithInitialBackoff(5 * time.Second))
	cfg.Default()

	assert.Equal(t, 5*time.Second, *cfg.InitialBackoff)
	assert.Equal(t, DefaultMaxBackoff, *cfg.MaxBackoff)

	backoff := cfg.GetBackoff()
	assert.NotNil(t, backoff)
}
