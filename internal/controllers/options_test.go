package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithInitialBackoff(t *testing.T) {
	t.Parallel()

	t.Run("configures initial backoff", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		customDuration := 25 * time.Second

		opt := WithInitialBackoff(customDuration)
		opt.ConfigureBackoff(config)

		require.NotNil(t, config.InitialBackoff)
		assert.Equal(t, customDuration, *config.InitialBackoff)
	})

	t.Run("overwrites existing initial backoff", func(t *testing.T) {
		t.Parallel()

		existingDuration := 10 * time.Second
		config := &BackoffConfig{
			InitialBackoff: &existingDuration,
		}

		newDuration := 50 * time.Second
		opt := WithInitialBackoff(newDuration)
		opt.ConfigureBackoff(config)

		assert.Equal(t, newDuration, *config.InitialBackoff)
	})

	t.Run("handles zero duration", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		opt := WithInitialBackoff(0)
		opt.ConfigureBackoff(config)

		require.NotNil(t, config.InitialBackoff)
		assert.Equal(t, time.Duration(0), *config.InitialBackoff)
	})

	t.Run("handles very long duration", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		longDuration := 24 * time.Hour
		opt := WithInitialBackoff(longDuration)
		opt.ConfigureBackoff(config)

		require.NotNil(t, config.InitialBackoff)
		assert.Equal(t, longDuration, *config.InitialBackoff)
	})
}

func TestWithMaxBackoff(t *testing.T) {
	t.Parallel()

	t.Run("configures max backoff", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		customDuration := 180 * time.Second

		opt := WithMaxBackoff(customDuration)
		opt.ConfigureBackoff(config)

		require.NotNil(t, config.MaxBackoff)
		assert.Equal(t, customDuration, *config.MaxBackoff)
	})

	t.Run("overwrites existing max backoff", func(t *testing.T) {
		t.Parallel()

		existingDuration := 300 * time.Second
		config := &BackoffConfig{
			MaxBackoff: &existingDuration,
		}

		newDuration := 600 * time.Second
		opt := WithMaxBackoff(newDuration)
		opt.ConfigureBackoff(config)

		assert.Equal(t, newDuration, *config.MaxBackoff)
	})

	t.Run("handles zero duration", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		opt := WithMaxBackoff(0)
		opt.ConfigureBackoff(config)

		require.NotNil(t, config.MaxBackoff)
		assert.Equal(t, time.Duration(0), *config.MaxBackoff)
	})

	t.Run("handles very long duration", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		longDuration := 7 * 24 * time.Hour // One week
		opt := WithMaxBackoff(longDuration)
		opt.ConfigureBackoff(config)

		require.NotNil(t, config.MaxBackoff)
		assert.Equal(t, longDuration, *config.MaxBackoff)
	})
}

func TestBackoffOptionsInterface(t *testing.T) {
	t.Parallel()

	t.Run("WithInitialBackoff implements BackoffOption", func(t *testing.T) {
		t.Parallel()

		var _ BackoffOption = WithInitialBackoff(10 * time.Second)
	})

	t.Run("WithMaxBackoff implements BackoffOption", func(t *testing.T) {
		t.Parallel()

		var _ BackoffOption = WithMaxBackoff(300 * time.Second)
	})
}

func TestBackoffOptions_Combined(t *testing.T) {
	t.Parallel()

	t.Run("both options work together", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		initialDuration := 15 * time.Second
		maxDuration := 200 * time.Second

		WithInitialBackoff(initialDuration).ConfigureBackoff(config)
		WithMaxBackoff(maxDuration).ConfigureBackoff(config)

		require.NotNil(t, config.InitialBackoff)
		require.NotNil(t, config.MaxBackoff)
		assert.Equal(t, initialDuration, *config.InitialBackoff)
		assert.Equal(t, maxDuration, *config.MaxBackoff)
	})

	t.Run("options can be applied in any order", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		initialDuration := 12 * time.Second
		maxDuration := 250 * time.Second

		// Apply max before initial
		WithMaxBackoff(maxDuration).ConfigureBackoff(config)
		WithInitialBackoff(initialDuration).ConfigureBackoff(config)

		require.NotNil(t, config.InitialBackoff)
		require.NotNil(t, config.MaxBackoff)
		assert.Equal(t, initialDuration, *config.InitialBackoff)
		assert.Equal(t, maxDuration, *config.MaxBackoff)
	})
}
