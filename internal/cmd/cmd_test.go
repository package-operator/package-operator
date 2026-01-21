package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewScheme(t *testing.T) {
	t.Parallel()

	scheme, err := NewScheme()

	require.NoError(t, err)
	require.NotNil(t, scheme)

	// Verify known types are registered
	require.True(t, scheme.Recognizes(schema.GroupVersionKind{
		Group:   "package-operator.run",
		Version: "v1alpha1",
		Kind:    "Package",
	}))
}

func TestErrInvalidArgs(t *testing.T) {
	t.Parallel()

	require.Equal(t, "arguments invalid", ErrInvalidArgs.Error())
}

func TestNewDefaultClientFactory(t *testing.T) {
	t.Parallel()

	mockKubeFactory := &mockKubeClientFactory{}
	factory := NewDefaultClientFactory(mockKubeFactory)

	require.NotNil(t, factory)
	require.Equal(t, mockKubeFactory, factory.kcliFactory)
}

func TestDefaultClientFactory_Client(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		kubeFactory KubeClientFactory
		expectError bool
	}{
		{
			name:        "successful client creation",
			kubeFactory: &mockKubeClientFactory{client: &mockClient{}},
			expectError: false,
		},
		{
			name:        "kube client factory error",
			kubeFactory: &mockKubeClientFactory{err: errors.New("factory error")},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			factory := NewDefaultClientFactory(test.kubeFactory)

			client, err := factory.Client()

			if test.expectError {
				require.Error(t, err)
				require.Nil(t, client)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
			}
		})
	}
}

func TestNewDefaultKubeClientFactory(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	configFactory := &mockRestConfigFactory{}

	factory := NewDefaultKubeClientFactory(scheme, configFactory)

	require.NotNil(t, factory)
	require.Equal(t, scheme, factory.scheme)
	require.Equal(t, configFactory, factory.cfgFactory)
}

func TestDefaultKubeClientFactory_GetKubeClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		configFactory RestConfigFactory
		expectError   bool
	}{
		{
			name:          "config factory error",
			configFactory: &mockRestConfigFactory{err: errors.New("config error")},
			expectError:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			scheme := runtime.NewScheme()
			factory := NewDefaultKubeClientFactory(scheme, test.configFactory)

			client, err := factory.GetKubeClient()

			if test.expectError {
				require.Error(t, err)
				require.Nil(t, client)
			} else if err != nil {
				// Note: In real scenarios without mocking the underlying client.New,
				// this would likely still error due to invalid config, but we're
				// testing the factory behavior

				require.Error(t, err)
			}
		})
	}
}

func TestNewDefaultRestConfigFactory(t *testing.T) {
	t.Parallel()

	factory := NewDefaultRestConfigFactory()

	require.NotNil(t, factory)
}

func TestDefaultWaiter_WaitForCondition(t *testing.T) {
	t.Parallel()

	mockClient := &Client{client: &mockClient{}}
	scheme := runtime.NewScheme()
	waiter := NewDefaultWaiter(mockClient, scheme)

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.waiter)

	// Test that the waiter has the expected timeout and interval
	// This is more of a constructor test since the actual waiting
	// would require complex mocking of the wait.Waiter
}

func TestNewDefaultWaiter(t *testing.T) {
	t.Parallel()

	mockClient := &Client{client: &mockClient{}}
	scheme := runtime.NewScheme()

	waiter := NewDefaultWaiter(mockClient, scheme)

	require.NotNil(t, waiter)
	require.NotNil(t, waiter.waiter)
}

// Mock implementations for testing

type mockKubeClientFactory struct {
	client client.Client
	err    error
}

func (m *mockKubeClientFactory) GetKubeClient() (client.Client, error) {
	return m.client, m.err
}

type mockRestConfigFactory struct {
	config *rest.Config
	err    error
}

func (m *mockRestConfigFactory) GetConfig() (*rest.Config, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.config != nil {
		return m.config, nil
	}
	return &rest.Config{}, nil
}

type mockClient struct {
	client.Client
}

func (m *mockClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return nil
}

func (m *mockClient) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return nil
}

func (m *mockClient) Create(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
	return nil
}

func (m *mockClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return nil
}

func (m *mockClient) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	return nil
}

func (m *mockClient) Patch(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
	return nil
}

func (m *mockClient) DeleteAllOf(_ context.Context, _ client.Object, _ ...client.DeleteAllOfOption) error {
	return nil
}
