package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackoffConfig_Default(t *testing.T) {
	t.Parallel()

	t.Run("sets defaults when nil", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		config.Default()

		require.NotNil(t, config.InitialBackoff)
		require.NotNil(t, config.MaxBackoff)
		assert.Equal(t, DefaultInitialBackoff, *config.InitialBackoff)
		assert.Equal(t, DefaultMaxBackoff, *config.MaxBackoff)
	})

	t.Run("preserves existing values", func(t *testing.T) {
		t.Parallel()

		customInitial := 5 * time.Second
		customMax := 60 * time.Second
		config := &BackoffConfig{
			InitialBackoff: &customInitial,
			MaxBackoff:     &customMax,
		}
		config.Default()

		assert.Equal(t, customInitial, *config.InitialBackoff)
		assert.Equal(t, customMax, *config.MaxBackoff)
	})

	t.Run("sets only missing values", func(t *testing.T) {
		t.Parallel()

		customInitial := 7 * time.Second
		config := &BackoffConfig{
			InitialBackoff: &customInitial,
		}
		config.Default()

		assert.Equal(t, customInitial, *config.InitialBackoff)
		assert.Equal(t, DefaultMaxBackoff, *config.MaxBackoff)
	})
}

func TestBackoffConfig_Option(t *testing.T) {
	t.Parallel()

	t.Run("applies single option", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		customInitial := 15 * time.Second
		config.Option(WithInitialBackoff(customInitial))

		require.NotNil(t, config.InitialBackoff)
		assert.Equal(t, customInitial, *config.InitialBackoff)
	})

	t.Run("applies multiple options", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		customInitial := 20 * time.Second
		customMax := 120 * time.Second
		config.Option(
			WithInitialBackoff(customInitial),
			WithMaxBackoff(customMax),
		)

		require.NotNil(t, config.InitialBackoff)
		require.NotNil(t, config.MaxBackoff)
		assert.Equal(t, customInitial, *config.InitialBackoff)
		assert.Equal(t, customMax, *config.MaxBackoff)
	})

	t.Run("no options applied", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		config.Option()

		assert.Nil(t, config.InitialBackoff)
		assert.Nil(t, config.MaxBackoff)
	})
}

func TestBackoffConfig_GetBackoff(t *testing.T) {
	t.Parallel()

	t.Run("creates backoff with configured values", func(t *testing.T) {
		t.Parallel()

		customInitial := 5 * time.Second
		customMax := 100 * time.Second
		config := &BackoffConfig{
			InitialBackoff: &customInitial,
			MaxBackoff:     &customMax,
		}

		backoff := config.GetBackoff()
		require.NotNil(t, backoff)

		// The flowcontrol.Backoff doesn't expose its config directly,
		// but we can verify it was created successfully
		assert.NotNil(t, backoff)
	})

	t.Run("creates backoff with default values", func(t *testing.T) {
		t.Parallel()

		config := &BackoffConfig{}
		config.Default()

		backoff := config.GetBackoff()
		require.NotNil(t, backoff)
	})
}

func TestBackoffConstants(t *testing.T) {
	t.Parallel()

	t.Run("default values are defined", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 30*time.Second, DefaultGlobalMissConfigurationRetry)
		assert.Equal(t, 10*time.Second, DefaultInitialBackoff)
		assert.Equal(t, 300*time.Second, DefaultMaxBackoff)
	})
}
