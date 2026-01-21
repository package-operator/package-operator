package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest
func TestImageRegistry(t *testing.T) {
	t.Run("default value", func(t *testing.T) {
		// Save original value
		originalValue, exists := os.LookupEnv(imageRegistryEnvvarName)
		defer func() {
			if exists {
				require.NoError(t, os.Setenv(imageRegistryEnvvarName, originalValue))
			} else {
				require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))
			}
		}()

		// Unset the environment variable
		require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))

		result := imageRegistry()
		assert.Equal(t, defaultImageRegistry, result)
	})

	t.Run("custom value from environment", func(t *testing.T) {
		// Save original value
		originalValue, exists := os.LookupEnv(imageRegistryEnvvarName)
		defer func() {
			if exists {
				require.NoError(t, os.Setenv(imageRegistryEnvvarName, originalValue))
			} else {
				require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))
			}
		}()

		customRegistry := "custom.registry.com/myapp"
		require.NoError(t, os.Setenv(imageRegistryEnvvarName, customRegistry))

		result := imageRegistry()
		assert.Equal(t, customRegistry, result)
	})
}

//nolint:paralleltest
func TestImageRegistryHost(t *testing.T) {
	tests := []struct {
		name         string
		registry     string
		expectedHost string
		shouldPanic  bool
	}{
		{
			name:         "quay.io registry",
			registry:     "quay.io/foobar",
			expectedHost: "quay.io",
		},
		{
			name:         "localhost registry",
			registry:     "localhost:5000/myapp",
			expectedHost: "localhost:5000",
		},
		{
			name:         "docker hub style",
			registry:     "docker.io/myuser/myapp",
			expectedHost: "docker.io",
		},
		{
			name:         "default registry",
			registry:     defaultImageRegistry,
			expectedHost: "dev.package-operator.run",
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			// Save original value for this subtest
			originalValue, exists := os.LookupEnv(imageRegistryEnvvarName)
			defer func() {
				if exists {
					require.NoError(t, os.Setenv(imageRegistryEnvvarName, originalValue))
				} else {
					require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))
				}
			}()

			require.NoError(t, os.Setenv(imageRegistryEnvvarName, test.registry))

			if test.shouldPanic {
				assert.Panics(t, func() {
					imageRegistryHost()
				})
				return
			}

			result := imageRegistryHost()
			assert.Equal(t, test.expectedHost, result)
		})
	}
}

//nolint:paralleltest
func TestImageRegistryNamespace(t *testing.T) {
	tests := []struct {
		name              string
		registry          string
		expectedNamespace string
	}{
		{
			name:              "quay.io registry",
			registry:          "quay.io/foobar",
			expectedNamespace: "foobar",
		},
		{
			name:              "localhost registry",
			registry:          "localhost:5000/myapp",
			expectedNamespace: "myapp",
		},
		{
			name:              "docker hub style with multiple levels",
			registry:          "docker.io/myuser/myapp",
			expectedNamespace: "myuser/myapp",
		},
		{
			name:              "default registry",
			registry:          defaultImageRegistry,
			expectedNamespace: "package-operator",
		},
		{
			name:              "registry without namespace",
			registry:          "docker.io",
			expectedNamespace: "",
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			// Save original value for this subtest
			originalValue, exists := os.LookupEnv(imageRegistryEnvvarName)
			defer func() {
				if exists {
					require.NoError(t, os.Setenv(imageRegistryEnvvarName, originalValue))
				} else {
					require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))
				}
			}()

			require.NoError(t, os.Setenv(imageRegistryEnvvarName, test.registry))

			result := imageRegistryNamespace()
			assert.Equal(t, test.expectedNamespace, result)
		})
	}
}

//nolint:paralleltest
func TestLocalRegistry(t *testing.T) {
	tests := []struct {
		name           string
		registry       string
		hostPort       string
		expectedResult string
	}{
		{
			name:           "quay.io registry with localhost",
			registry:       "quay.io/foobar",
			hostPort:       "localhost:3001",
			expectedResult: "localhost:3001/foobar",
		},
		{
			name:           "custom registry with different host port",
			registry:       "docker.io/myuser/myapp",
			hostPort:       "127.0.0.1:5000",
			expectedResult: "127.0.0.1:5000/myuser/myapp",
		},
		{
			name:           "default registry",
			registry:       defaultImageRegistry,
			hostPort:       "localhost:5001",
			expectedResult: "localhost:5001/package-operator",
		},
	}

	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			// Save original value for this subtest
			originalValue, exists := os.LookupEnv(imageRegistryEnvvarName)
			defer func() {
				if exists {
					require.NoError(t, os.Setenv(imageRegistryEnvvarName, originalValue))
				} else {
					require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))
				}
			}()

			require.NoError(t, os.Setenv(imageRegistryEnvvarName, test.registry))

			result := localRegistry(test.hostPort)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ".cache", cacheDir)
	assert.Equal(t, "dev.package-operator.run/package-operator", defaultImageRegistry)
	assert.Equal(t, "IMAGE_REGISTRY", imageRegistryEnvvarName)
	assert.Equal(t, int32(5001), devClusterRegistryPort)
	assert.Equal(t, int32(5002), devClusterRegistryAuthPort)
}

//nolint:paralleltest
func TestImageRegistryHost_EdgeCases(t *testing.T) {
	t.Run("with port", func(t *testing.T) {
		// Save original value for this subtest
		originalValue, exists := os.LookupEnv(imageRegistryEnvvarName)
		defer func() {
			if exists {
				require.NoError(t, os.Setenv(imageRegistryEnvvarName, originalValue))
			} else {
				require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))
			}
		}()

		require.NoError(t, os.Setenv(imageRegistryEnvvarName, "registry.example.com:8080/namespace"))

		result := imageRegistryHost()
		assert.Equal(t, "registry.example.com:8080", result)
	})

	t.Run("simple hostname", func(t *testing.T) {
		// Save original value for this subtest
		originalValue, exists := os.LookupEnv(imageRegistryEnvvarName)
		defer func() {
			if exists {
				require.NoError(t, os.Setenv(imageRegistryEnvvarName, originalValue))
			} else {
				require.NoError(t, os.Unsetenv(imageRegistryEnvvarName))
			}
		}()

		require.NoError(t, os.Setenv(imageRegistryEnvvarName, "simple"))

		result := imageRegistryHost()
		assert.Equal(t, "simple", result)
	})
}
