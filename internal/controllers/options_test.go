package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithInitialBackoff_ConfigureBackoff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "5 seconds",
			duration: 5 * time.Second,
		},
		{
			name:     "1 minute",
			duration: 1 * time.Minute,
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
		},
		{
			name:     "zero duration",
			duration: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			option := WithInitialBackoff(test.duration)
			config := &BackoffConfig{}

			option.ConfigureBackoff(config)

			assert.NotNil(t, config.InitialBackoff)
			assert.Equal(t, test.duration, *config.InitialBackoff)
		})
	}
}

func TestWithMaxBackoff_ConfigureBackoff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "5 minutes",
			duration: 5 * time.Minute,
		},
		{
			name:     "10 minutes",
			duration: 10 * time.Minute,
		},
		{
			name:     "1 hour",
			duration: 1 * time.Hour,
		},
		{
			name:     "zero duration",
			duration: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			option := WithMaxBackoff(test.duration)
			config := &BackoffConfig{}

			option.ConfigureBackoff(config)

			assert.NotNil(t, config.MaxBackoff)
			assert.Equal(t, test.duration, *config.MaxBackoff)
		})
	}
}

func TestWithInitialBackoff_Type(t *testing.T) {
	t.Parallel()

	duration := 15 * time.Second
	option := WithInitialBackoff(duration)

	// Verify the option implements BackoffOption
	var _ BackoffOption = option

	// Verify the underlying type conversion
	assert.Equal(t, time.Duration(option), duration)
}

func TestWithMaxBackoff_Type(t *testing.T) {
	t.Parallel()

	duration := 2 * time.Minute
	option := WithMaxBackoff(duration)

	// Verify the option implements BackoffOption
	var _ BackoffOption = option

	// Verify the underlying type conversion
	assert.Equal(t, time.Duration(option), duration)
}

func TestWithInitialBackoff_IntegrationWithConfig(t *testing.T) {
	t.Parallel()

	initialBackoff := 7 * time.Second
	config := &BackoffConfig{}

	// Test the full flow: create option, apply to config, check default
	config.Option(WithInitialBackoff(initialBackoff))
	config.Default()

	assert.Equal(t, initialBackoff, *config.InitialBackoff)
	assert.Equal(t, DefaultMaxBackoff, *config.MaxBackoff)
}

func TestWithMaxBackoff_IntegrationWithConfig(t *testing.T) {
	t.Parallel()

	maxBackoff := 2 * time.Minute
	config := &BackoffConfig{}

	// Test the full flow: create option, apply to config, check default
	config.Option(WithMaxBackoff(maxBackoff))
	config.Default()

	assert.Equal(t, DefaultInitialBackoff, *config.InitialBackoff)
	assert.Equal(t, maxBackoff, *config.MaxBackoff)
}

func TestWithBothOptions_IntegrationWithConfig(t *testing.T) {
	t.Parallel()

	initialBackoff := 3 * time.Second
	maxBackoff := 90 * time.Second
	config := &BackoffConfig{}

	// Test using both options together
	config.Option(
		WithInitialBackoff(initialBackoff),
		WithMaxBackoff(maxBackoff),
	)
	config.Default()

	assert.Equal(t, initialBackoff, *config.InitialBackoff)
	assert.Equal(t, maxBackoff, *config.MaxBackoff)
}

func TestOptions_OverrideValues(t *testing.T) {
	t.Parallel()

	config := &BackoffConfig{}

	// Apply first set of options
	config.Option(
		WithInitialBackoff(5*time.Second),
		WithMaxBackoff(60*time.Second),
	)

	// Override with new values
	config.Option(
		WithInitialBackoff(10*time.Second),
		WithMaxBackoff(120*time.Second),
	)

	assert.Equal(t, 10*time.Second, *config.InitialBackoff)
	assert.Equal(t, 120*time.Second, *config.MaxBackoff)
}
