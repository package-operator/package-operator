package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveDigestConfig_Option(t *testing.T) {
	t.Parallel()

	cfg := &ResolveDigestConfig{}

	cfg.Option(WithInsecure(true))

	assert.True(t, cfg.Insecure)
}

func TestResolveDigestConfig_Option_Multiple(t *testing.T) {
	t.Parallel()

	cfg := &ResolveDigestConfig{}

	cfg.Option(
		WithInsecure(true),
		WithInsecure(false), // Second option should override first
	)

	assert.False(t, cfg.Insecure)
}

func TestDefaultDigestResolver_ResolveDigest_Insecure(t *testing.T) {
	t.Parallel()

	resolver := &defaultDigestResolver{}

	// Test with invalid reference to ensure we get the expected error format
	// without actually making network calls
	_, err := resolver.ResolveDigest("invalid-reference", WithInsecure(true))

	// We expect an error since we're using an invalid reference
	assert.Error(t, err)
}

func TestDefaultDigestResolver_ResolveDigest_Secure(t *testing.T) {
	t.Parallel()

	resolver := &defaultDigestResolver{}

	// Test with invalid reference to ensure we get the expected error format
	// without actually making network calls
	_, err := resolver.ResolveDigest("invalid-reference", WithInsecure(false))

	// We expect an error since we're using an invalid reference
	assert.Error(t, err)
}

func TestDefaultDigestResolver_ResolveDigest_NoOptions(t *testing.T) {
	t.Parallel()

	resolver := &defaultDigestResolver{}

	// Test with invalid reference to ensure we get the expected error format
	// without actually making network calls
	_, err := resolver.ResolveDigest("invalid-reference")

	// We expect an error since we're using an invalid reference
	assert.Error(t, err)
}

func TestWithInsecure_ConfigureResolveDigest_Resolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		insecure bool
	}{
		{
			name:     "insecure true",
			insecure: true,
		},
		{
			name:     "insecure false",
			insecure: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			option := WithInsecure(test.insecure)
			config := &ResolveDigestConfig{}

			option.ConfigureResolveDigest(config)

			assert.Equal(t, test.insecure, config.Insecure)
		})
	}
}

func TestResolveDigestConfig_DefaultBehavior(t *testing.T) {
	t.Parallel()

	cfg := &ResolveDigestConfig{}

	// Default should be secure (false)
	assert.False(t, cfg.Insecure)
}
