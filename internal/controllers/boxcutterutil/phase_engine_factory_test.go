package boxcutterutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/openapi"
	"pkg.package-operator.run/boxcutter/validation"

	"package-operator.run/internal/testutil/managedcachemocks"
)

type discoveryClientMock struct {
	mock.Mock
}

func (m *discoveryClientMock) OpenAPIV3() openapi.Client {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(openapi.Client)
}

func (m *discoveryClientMock) ServerVersion() (*version.Info, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*version.Info), args.Error(1)
}

type restMapperMock struct {
	mock.Mock
	meta.RESTMapper
}

func TestNewPhaseEngineFactory(t *testing.T) {
	t.Parallel()

	t.Run("creates factory with all dependencies", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		discoveryClient := &discoveryClientMock{}
		restMapper := &restMapperMock{}
		phaseValidator := &validation.PhaseValidator{}

		factory := NewPhaseEngineFactory(
			scheme,
			discoveryClient,
			restMapper,
			phaseValidator,
		)

		require.NotNil(t, factory)
		assert.IsType(t, phaseEngineFactory{}, factory)

		// Verify internal fields are set
		f, ok := factory.(phaseEngineFactory)
		require.True(t, ok)
		assert.Equal(t, scheme, f.scheme)
		assert.Equal(t, discoveryClient, f.discoveryClient)
		assert.Equal(t, restMapper, f.restMapper)
		assert.Equal(t, phaseValidator, f.phaseValidator)
	})

	t.Run("creates factory with nil scheme", func(t *testing.T) {
		t.Parallel()

		discoveryClient := &discoveryClientMock{}
		restMapper := &restMapperMock{}
		phaseValidator := &validation.PhaseValidator{}

		factory := NewPhaseEngineFactory(
			nil,
			discoveryClient,
			restMapper,
			phaseValidator,
		)

		require.NotNil(t, factory)
		f, ok := factory.(phaseEngineFactory)
		require.True(t, ok)
		assert.Nil(t, f.scheme)
	})

	t.Run("creates factory with nil discoveryClient", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		restMapper := &restMapperMock{}
		phaseValidator := &validation.PhaseValidator{}

		factory := NewPhaseEngineFactory(
			scheme,
			nil,
			restMapper,
			phaseValidator,
		)

		require.NotNil(t, factory)
		f, ok := factory.(phaseEngineFactory)
		require.True(t, ok)
		assert.Nil(t, f.discoveryClient)
	})

	t.Run("creates factory with nil restMapper", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		discoveryClient := &discoveryClientMock{}
		phaseValidator := &validation.PhaseValidator{}

		factory := NewPhaseEngineFactory(
			scheme,
			discoveryClient,
			nil,
			phaseValidator,
		)

		require.NotNil(t, factory)
		f, ok := factory.(phaseEngineFactory)
		require.True(t, ok)
		assert.Nil(t, f.restMapper)
	})

	t.Run("creates factory with nil phaseValidator", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		discoveryClient := &discoveryClientMock{}
		restMapper := &restMapperMock{}

		factory := NewPhaseEngineFactory(
			scheme,
			discoveryClient,
			restMapper,
			nil,
		)

		require.NotNil(t, factory)
		f, ok := factory.(phaseEngineFactory)
		require.True(t, ok)
		assert.Nil(t, f.phaseValidator)
	})
}

func TestPhaseEngineFactory_New(t *testing.T) {
	t.Parallel()

	t.Run("creates phase engine successfully", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		discoveryClient := &discoveryClientMock{}
		discoveryClient.On("OpenAPIV3").Return(nil)
		restMapper := &restMapperMock{}
		phaseValidator := &validation.PhaseValidator{}
		accessor := &managedcachemocks.AccessorMock{}

		factory := NewPhaseEngineFactory(
			scheme,
			discoveryClient,
			restMapper,
			phaseValidator,
		)

		engine, err := factory.New(accessor)

		require.NoError(t, err)
		require.NotNil(t, engine)
	})

	t.Run("creates phase engine with nil accessor", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		discoveryClient := &discoveryClientMock{}
		discoveryClient.On("OpenAPIV3").Return(nil)
		restMapper := &restMapperMock{}
		phaseValidator := &validation.PhaseValidator{}

		factory := NewPhaseEngineFactory(
			scheme,
			discoveryClient,
			restMapper,
			phaseValidator,
		)

		engine, err := factory.New(nil)

		// Creating a phase engine with nil accessor should fail
		require.Error(t, err)
		assert.Nil(t, engine)
	})

	t.Run("multiple calls create separate instances", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		discoveryClient := &discoveryClientMock{}
		discoveryClient.On("OpenAPIV3").Return(nil)
		restMapper := &restMapperMock{}
		phaseValidator := &validation.PhaseValidator{}
		accessor1 := &managedcachemocks.AccessorMock{}
		accessor2 := &managedcachemocks.AccessorMock{}

		factory := NewPhaseEngineFactory(
			scheme,
			discoveryClient,
			restMapper,
			phaseValidator,
		)

		engine1, err1 := factory.New(accessor1)
		require.NoError(t, err1)
		require.NotNil(t, engine1)

		engine2, err2 := factory.New(accessor2)
		require.NoError(t, err2)
		require.NotNil(t, engine2)

		// Verify they are different instances
		assert.NotSame(t, engine1, engine2)
	})
}

func TestPhaseEngineFactoryInterface(t *testing.T) {
	t.Parallel()

	t.Run("implements PhaseEngineFactory interface", func(t *testing.T) {
		t.Parallel()

		scheme := runtime.NewScheme()
		discoveryClient := &discoveryClientMock{}
		restMapper := &restMapperMock{}
		phaseValidator := &validation.PhaseValidator{}

		_ = NewPhaseEngineFactory(
			scheme,
			discoveryClient,
			restMapper,
			phaseValidator,
		)
	})
}

func TestPhaseEngineFactory_WithMinimalDependencies(t *testing.T) {
	t.Parallel()

	t.Run("creates engine with all nil dependencies", func(t *testing.T) {
		t.Parallel()

		// Creating an engine with all nil dependencies should fail
		// because the boxcutter engine requires a scheme
		factory := NewPhaseEngineFactory(nil, nil, nil, nil)
		accessor := &managedcachemocks.AccessorMock{}

		engine, err := factory.New(accessor)

		require.Error(t, err)
		assert.Nil(t, engine)
	})
}

func TestDiscoveryClientInterface(t *testing.T) {
	t.Parallel()

	t.Run("mock implements DiscoveryClient interface", func(t *testing.T) {
		t.Parallel()

		var _ DiscoveryClient = &discoveryClientMock{}
	})

	t.Run("OpenAPIV3 returns nil", func(t *testing.T) {
		t.Parallel()

		mock := &discoveryClientMock{}
		mock.On("OpenAPIV3").Return(nil)

		result := mock.OpenAPIV3()
		assert.Nil(t, result)
	})

	t.Run("ServerVersion returns version", func(t *testing.T) {
		t.Parallel()

		mock := &discoveryClientMock{}
		expectedVersion := &version.Info{
			Major: "1",
			Minor: "28",
		}
		mock.On("ServerVersion").Return(expectedVersion, nil)

		result, err := mock.ServerVersion()
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, result)
	})

	t.Run("ServerVersion returns error", func(t *testing.T) {
		t.Parallel()

		mock := &discoveryClientMock{}
		expectedErr := assert.AnError
		mock.On("ServerVersion").Return(nil, expectedErr)

		result, err := mock.ServerVersion()
		assert.Nil(t, result)
		assert.Equal(t, expectedErr, err)
	})
}
