package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewComponents(t *testing.T) {
	t.Parallel()
	_, err := NewComponents()
	require.NoError(t, err)
}

func TestProvideScheme(t *testing.T) {
	t.Parallel()
	_, err := ProvideScheme()
	require.NoError(t, err)
}

func TestProvideLogger(t *testing.T) {
	t.Parallel()
	_ = ProvideLogger()
}

func TestProvideMetricsRecorder(t *testing.T) {
	t.Parallel()
	_ = ProvideMetricsRecorder()
}

func TestUncachedClient(t *testing.T) {
	t.Parallel()
	_, err := ProvideUncachedClient(nil, nil)
	require.EqualError(t, err,
		"unable to set up uncached client: must provide non-nil rest.Config to client.New")
}
