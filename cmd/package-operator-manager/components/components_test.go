package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewComponents(t *testing.T) {
	_, err := NewComponents()
	require.NoError(t, err)
}

func TestProvideScheme(t *testing.T) {
	_, err := ProvideScheme()
	require.NoError(t, err)
}

func TestProvideLogger(_ *testing.T) {
	_ = ProvideLogger()
}

func TestProvideMetricsRecorder(_ *testing.T) {
	_ = ProvideMetricsRecorder()
}

func TestUncachedClient(t *testing.T) {
	_, err := ProvideUncachedClient(nil, nil)
	require.EqualError(t, err,
		"unable to set up uncached client: must provide non-nil rest.Config to client.New")
}
