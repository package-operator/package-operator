package components

import (
	"fmt"

	"go.uber.org/dig"
	ctrl "sigs.k8s.io/controller-runtime"
)

type controllerSetup struct {
	name       string
	controller controller
}

func setupAll(mgr ctrl.Manager, controllers []controllerSetup) error {
	for _, c := range controllers {
		if err := c.controller.SetupWithManager(mgr); err != nil {
			return fmt.Errorf(
				"unable to create controller for %s: %w", c.name, err)
		}
	}
	return nil
}

// interface implemented by all controllers.
type controller interface {
	SetupWithManager(mgr ctrl.Manager) error
}

// DI container to get all controllers.
type AllControllers struct {
	dig.In

	ObjectSet        ObjectSetController
	ClusterObjectSet ClusterObjectSetController

	ObjectSetPhase        ObjectSetPhaseController
	ClusterObjectSetPhase ClusterObjectSetPhaseController

	ObjectDeployment        ObjectDeploymentController
	ClusterObjectDeployment ClusterObjectDeploymentController

	Package        PackageController
	ClusterPackage ClusterPackageController

	ObjectTemplate        ObjectTemplateController
	ClusterObjectTemplate ClusterObjectTemplateController
}

func (ac AllControllers) List() []interface{} {
	return []interface{}{
		ac.ObjectSet, ac.ClusterObjectSet,
		ac.ObjectSetPhase, ac.ClusterObjectSetPhase,
		ac.ObjectDeployment, ac.ClusterObjectDeployment,
		ac.Package, ac.ClusterPackage,
		ac.ObjectTemplate, ac.ClusterObjectTemplate,
	}
}

func (ac AllControllers) SetupWithManager(mgr ctrl.Manager) error {
	return setupAll(mgr, []controllerSetup{
		{
			name:       "ObjectSet",
			controller: ac.ObjectSet,
		},
		{
			name:       "ClusterObjectSet",
			controller: ac.ClusterObjectSet,
		},
		{
			name:       "ObjectSetPhase",
			controller: ac.ObjectSetPhase,
		},
		{
			name:       "ClusterObjectSetPhase",
			controller: ac.ClusterObjectSetPhase,
		},
		{
			name:       "ObjectDeployment",
			controller: ac.ObjectDeployment,
		},
		{
			name:       "ClusterObjectDeployment",
			controller: ac.ClusterObjectDeployment,
		},
		{
			name:       "Package",
			controller: ac.Package,
		},
		{
			name:       "ClusterPackage",
			controller: ac.ClusterPackage,
		},
		{
			name:       "ObjectTemplate",
			controller: ac.ObjectTemplate,
		},
		{
			name:       "ClusterObjectTemplate",
			controller: ac.ClusterObjectTemplate,
		},
	})
}

// DI container to get only the controllers needed for self-bootstrap.
type BootstrapControllers struct {
	dig.In

	ClusterPackage          ClusterPackageController
	ClusterObjectDeployment ClusterObjectDeploymentController
	ClusterObjectSet        ClusterObjectSetController
}

func (bc BootstrapControllers) SetupWithManager(mgr ctrl.Manager) error {
	return setupAll(mgr, []controllerSetup{
		{
			name:       "ClusterObjectSet",
			controller: bc.ClusterObjectSet,
		},
		{
			name:       "ClusterObjectDeployment",
			controller: bc.ClusterObjectDeployment,
		},
		{
			name:       "ClusterPackage",
			controller: bc.ClusterPackage,
		},
	})
}
