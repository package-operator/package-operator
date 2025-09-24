package cmd

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"package-operator.run/internal/packages"
)

func TestWithClock_ConfigureUpdate(t *testing.T) {
	t.Parallel()

	mockClock := &mockClock{}
	option := WithClock{Clock: mockClock}
	config := &UpdateConfig{}

	option.ConfigureUpdate(config)

	assert.Equal(t, mockClock, config.Clock)
}

func TestWithClusterScope_ConfigureRenderPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		clusterScope bool
	}{
		{
			name:         "cluster scope true",
			clusterScope: true,
		},
		{
			name:         "cluster scope false",
			clusterScope: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			option := WithClusterScope(test.clusterScope)
			config := &RenderPackageConfig{}

			option.ConfigureRenderPackage(config)

			assert.Equal(t, test.clusterScope, config.ClusterScope)
		})
	}
}

func TestWithConfigPath_ConfigureRenderPackage(t *testing.T) {
	t.Parallel()

	configPath := "/test/config/path"
	option := WithConfigPath(configPath)
	config := &RenderPackageConfig{}

	option.ConfigureRenderPackage(config)

	assert.Equal(t, configPath, config.ConfigPath)
}

func TestWithConfigTestcase_ConfigureRenderPackage(t *testing.T) {
	t.Parallel()

	testcase := "test-case-name"
	option := WithConfigTestcase(testcase)
	config := &RenderPackageConfig{}

	option.ConfigureRenderPackage(config)

	assert.Equal(t, testcase, config.ConfigTestcase)
}

func TestWithComponent_ConfigureRenderPackage(t *testing.T) {
	t.Parallel()

	component := "test-component"
	option := WithComponent(component)
	config := &RenderPackageConfig{}

	option.ConfigureRenderPackage(config)

	assert.Equal(t, component, config.Component)
}

func TestWithLog_ConfigureTree(t *testing.T) {
	t.Parallel()

	logger := logr.Discard()
	option := WithLog{Log: logger}
	config := &TreeConfig{}

	option.ConfigureTree(config)

	assert.Equal(t, logger, config.Log)
}

func TestWithLog_ConfigureUpdate(t *testing.T) {
	t.Parallel()

	logger := logr.Discard()
	option := WithLog{Log: logger}
	config := &UpdateConfig{}

	option.ConfigureUpdate(config)

	assert.Equal(t, logger, config.Log)
}

func TestWithLog_ConfigureValidate(t *testing.T) {
	t.Parallel()

	logger := logr.Discard()
	option := WithLog{Log: logger}
	config := &ValidateConfig{}

	option.ConfigureValidate(config)

	assert.Equal(t, logger, config.Log)
}

func TestWithHeaders_ConfigureTable(t *testing.T) {
	t.Parallel()

	headers := []string{"header1", "header2", "header3"}
	option := WithHeaders(headers)
	config := &TableConfig{}

	option.ConfigureTable(config)

	assert.Equal(t, headers, config.Headers)
}

func TestWithInsecure_ConfigureBuildFromSource(t *testing.T) {
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
			config := &BuildFromSourceConfig{}

			option.ConfigureBuildFromSource(config)

			assert.Equal(t, test.insecure, config.Insecure)
		})
	}
}

func TestWithInsecure_ConfigureGenerateLockData(t *testing.T) {
	t.Parallel()

	option := WithInsecure(true)
	config := &GenerateLockDataConfig{}

	option.ConfigureGenerateLockData(config)

	assert.True(t, config.Insecure)
}

func TestWithInsecure_ConfigureResolveDigest(t *testing.T) {
	t.Parallel()

	option := WithInsecure(true)
	config := &ResolveDigestConfig{}

	option.ConfigureResolveDigest(config)

	assert.True(t, config.Insecure)
}

func TestWithInsecure_ConfigureValidatePackage(t *testing.T) {
	t.Parallel()

	option := WithInsecure(true)
	config := &ValidatePackageConfig{}

	option.ConfigureValidatePackage(config)

	assert.True(t, config.Insecure)
}

func TestWithNamespace_ConfigureGetPackage(t *testing.T) {
	t.Parallel()

	namespace := "test-namespace"
	option := WithNamespace(namespace)
	config := &GetPackageConfig{}

	option.ConfigureGetPackage(config)

	assert.Equal(t, namespace, config.Namespace)
}

func TestWithNamespace_ConfigureGetObjectDeployment(t *testing.T) {
	t.Parallel()

	namespace := "test-namespace"
	option := WithNamespace(namespace)
	config := &GetObjectDeploymentConfig{}

	option.ConfigureGetObjectDeployment(config)

	assert.Equal(t, namespace, config.Namespace)
}

func TestWithOutputPath_ConfigureBuildFromSource(t *testing.T) {
	t.Parallel()

	outputPath := "/test/output/path"
	option := WithOutputPath(outputPath)
	config := &BuildFromSourceConfig{}

	option.ConfigureBuildFromSource(config)

	assert.Equal(t, outputPath, config.OutputPath)
}

func TestWithPackageLoader_ConfigureUpdate(t *testing.T) {
	t.Parallel()

	loader := &mockPackageLoader{}
	option := WithPackageLoader{Loader: loader}
	config := &UpdateConfig{}

	option.ConfigureUpdate(config)

	assert.Equal(t, loader, config.Loader)
}

func TestWithPath_ConfigureValidatePackage(t *testing.T) {
	t.Parallel()

	path := "/test/path"
	option := WithPath(path)
	config := &ValidatePackageConfig{}

	option.ConfigureValidatePackage(config)

	assert.Equal(t, path, config.Path)
}

func TestWithPush_ConfigureBuildFromSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		push bool
	}{
		{
			name: "push true",
			push: true,
		},
		{
			name: "push false",
			push: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			option := WithPush(test.push)
			config := &BuildFromSourceConfig{}

			option.ConfigureBuildFromSource(config)

			assert.Equal(t, test.push, config.Push)
		})
	}
}

func TestWithRemoteReference_ConfigureValidatePackage(t *testing.T) {
	t.Parallel()

	remoteRef := "registry.example.com/package:v1.0.0"
	option := WithRemoteReference(remoteRef)
	config := &ValidatePackageConfig{}

	option.ConfigureValidatePackage(config)

	assert.Equal(t, remoteRef, config.RemoteReference)
}

func TestWithTags_ConfigureBuildFromSource(t *testing.T) {
	t.Parallel()

	existingTags := []string{"existing1", "existing2"}
	newTags := []string{"new1", "new2"}
	option := WithTags(newTags)
	config := &BuildFromSourceConfig{Tags: existingTags}

	option.ConfigureBuildFromSource(config)

	expectedTags := append(existingTags, newTags...) //nolint:gocritic
	assert.Equal(t, expectedTags, config.Tags)
}

// Mock implementations for testing

type mockClock struct{}

func (m *mockClock) Now() metav1.Time {
	return metav1.Now()
}

type mockPackageLoader struct{}

func (m *mockPackageLoader) LoadPackage(_ context.Context, _ string) (*packages.Package, error) {
	return &packages.Package{}, nil
}
