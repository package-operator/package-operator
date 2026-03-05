package components

import (
	"os"
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

func TestProvideRestConfig(t *testing.T) {
	// Create temporary empty kubeconfig file and point KUBECONFIG to it.
	// Otherise this test can accidentally pick up one of:
	// - current value of KUBECONFIG
	// - default kubeconfig at ~/.kube/config
	f, err := os.CreateTemp("", "empty-kubeconfig-")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	defer func() {
		require.NoError(t, os.Remove(f.Name())) //nolint:gosec // G703: Safe - path is from os.CreateTemp
	}()

	t.Setenv("KUBECONFIG", f.Name())

	_, err = ProvideRestConfig()
	require.EqualError(t, err, "invalid configuration: no configuration has been provided"+
		", try setting KUBERNETES_MASTER environment variable")
}

func TestProvideManager(t *testing.T) {
	t.Parallel()

	_, err := ProvideManager(nil, nil, Options{})
	require.EqualError(t, err, "must specify Config")
}
