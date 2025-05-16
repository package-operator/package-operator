package controllersmocks

import (
	"github.com/stretchr/testify/mock"
	"pkg.package-operator.run/boxcutter/managedcache"

	"package-operator.run/internal/controllers"
)

type PhaseReconcilerFactoryMock struct {
	mock.Mock
}

func (m *PhaseReconcilerFactoryMock) New(accessor managedcache.Accessor) controllers.PhaseReconciler {
	args := m.Called(accessor)
	return args.Get(0).(controllers.PhaseReconciler)
}
