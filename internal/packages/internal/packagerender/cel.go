package packagerender

import "package-operator.run/internal/packages/internal/packagetypes"

func evaluateCELCondition(cel string, tmplCtx packagetypes.PackageRenderContext) (bool, error) {
	return true, nil
}
