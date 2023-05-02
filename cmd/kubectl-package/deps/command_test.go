package deps

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDefaultBuilderFactory(t *testing.T) {
	t.Parallel()

	logFactoryMock := &logFactoryMock{}
	logFactoryMock.On("Logger").Return(logr.Discard())

	factory := &defaultBuilderFactory{
		scheme:     runtime.NewScheme(),
		logFactory: logFactoryMock,
	}

	require.NotNil(t, factory.Builder())
}

func TestDefaultRendererFactory(t *testing.T) {
	t.Parallel()

	logFactoryMock := &logFactoryMock{}
	logFactoryMock.On("Logger").Return(logr.Discard())

	factory := &defaultRendererFactory{
		scheme:     runtime.NewScheme(),
		logFactory: logFactoryMock,
	}

	require.NotNil(t, factory.Renderer())
}

type logFactoryMock struct {
	mock.Mock
}

func (m *logFactoryMock) Logger() logr.Logger {
	args := m.Called()

	return args.Get(0).(logr.Logger)
}
