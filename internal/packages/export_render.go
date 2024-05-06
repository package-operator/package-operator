package packages

import "package-operator.run/internal/packages/internal/packagerender"

var (
	// Runs a go-template transformer on all .gotmpl files.
	RenderTemplates = packagerender.RenderTemplates
	// Renders all .yml and .yaml files into Kubernetes Objects.
	RenderObjects = packagerender.RenderObjects
	// Renders all .yml and .yaml files into Kubernetes Objects and applies CEL conditionals to filter objects.
	RenderObjectsWithFilter = packagerender.RenderObjectsWithFilter
	// Renders a ObjectSetTemplateSpec from a PackageInstance to use with ObjectSet and ObjectDeployment APIs.
	RenderObjectSetTemplateSpec = packagerender.RenderObjectSetTemplateSpec
	// Turns a Package and PackageRenderContext into a PackageInstance.
	RenderPackageInstance = packagerender.RenderPackageInstance
)
