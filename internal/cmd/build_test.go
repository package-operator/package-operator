package cmd

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
)

func TestBuildValidationError_Error(t *testing.T) {
	t.Parallel()

	err := BuildValidationError{Msg: "test error message"}
	require.Equal(t, "test error message", err.Error())
}

func TestNewBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []BuildOption
	}{
		{
			name: "no options",
			opts: nil,
		},
		{
			name: "with log option",
			opts: []BuildOption{WithLog{Log: logr.Discard()}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := NewBuild(test.opts...)
			require.NotNil(t, build)
			require.NotNil(t, build.cfg)
		})
	}
}

func TestBuildConfig_Option(t *testing.T) {
	t.Parallel()

	cfg := &BuildConfig{}
	logger := logr.Discard()

	cfg.Option(WithLog{Log: logger})

	require.Equal(t, logger, cfg.Log)
}

func TestBuildConfig_Default(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  BuildConfig
	}{
		{
			name: "empty config gets defaults",
			cfg:  BuildConfig{},
		},
		{
			name: "config with log preserves log",
			cfg:  BuildConfig{Log: logr.Discard()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := test.cfg
			cfg.Default()

			// Should have a logger (either provided or discard)
			require.NotNil(t, cfg.Log)

			// Should have a resolver
			require.NotNil(t, cfg.Resolver)
		})
	}
}

func TestBuildFromSourceConfig_Option(t *testing.T) {
	t.Parallel()

	cfg := &BuildFromSourceConfig{}

	cfg.Option(
		WithInsecure(true),
		WithOutputPath("/test/path"),
		WithTags([]string{"tag1", "tag2"}),
		WithPush(true),
	)

	require.True(t, cfg.Insecure)
	require.Equal(t, "/test/path", cfg.OutputPath)
	require.Equal(t, []string{"tag1", "tag2"}, cfg.Tags)
	require.True(t, cfg.Push)
}

func TestBuild_BuildFromSource_InvalidPath(t *testing.T) {
	t.Parallel()

	build := NewBuild()
	ctx := context.Background()

	err := build.BuildFromSource(ctx, "/nonexistent/path")

	require.Error(t, err)
	require.Contains(t, err.Error(), "load source from disk path")
}
