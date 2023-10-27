package packagestructure

import (
	"context"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/internal/packages/internal/packagetypes"
)

// StructuralLoader parses the raw package structure to produce something usable.
type StructuralLoader struct {
	scheme *runtime.Scheme
}

// Creates a new StructuralLoaderInstance.
func NewStructuralLoader(scheme *runtime.Scheme) *StructuralLoader {
	return &StructuralLoader{
		scheme: scheme,
	}
}

// Load a Package and it's sub-component Packages.
func (l *StructuralLoader) Load(
	ctx context.Context, rawPkg *packagetypes.RawPackage,
) (*packagetypes.Package, error) {
	return l.load(ctx, rawPkg.Files, "")
}

// Load a Sub-Component Package directly ignoring the root-package and any other sub component.
// Empty componentName represents just the root Package, excluding all individual components.
func (l *StructuralLoader) LoadComponent(
	ctx context.Context, rawPkg *packagetypes.RawPackage, componentName string,
) (*packagetypes.Package, error) {
	pkg, err := l.load(ctx, rawPkg.Files, "")
	if err != nil {
		return nil, err
	}

	if pkg.Manifest.Spec.Components == nil {
		if len(componentName) == 0 {
			return pkg, nil
		}
		return nil, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonComponentsNotEnabled,
		}
	}

	if len(componentName) == 0 {
		pkg.Components = nil
		return pkg, nil
	}

	for _, componentPkg := range pkg.Components {
		if componentPkg.Manifest.Name == componentName {
			return &componentPkg, nil
		}
	}

	return nil, packagetypes.ViolationError{
		Reason:    packagetypes.ViolationReasonComponentNotFound,
		Component: componentName,
	}
}

func (l *StructuralLoader) load(ctx context.Context, files packagetypes.Files, componentName string) (*packagetypes.Package, error) {
	pkg := &packagetypes.Package{}

	// PackageManifest
	if bothExtensions(files, packagetypes.PackageManifestFilename) {
		return nil, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonPackageManifestDuplicated,
		}
	}
	var err error
	manifestBytes, manifestPath, manifestFound := getFile(files, packagetypes.PackageManifestFilename)
	if !manifestFound {
		return nil, packagetypes.ErrManifestNotFound
	}
	pkg.Manifest, err = manifestFromFile(ctx, l.scheme, manifestPath, manifestBytes)
	if err != nil {
		return nil, err
	}
	if len(componentName) > 0 {
		pkg.Manifest.Name = componentName
	}

	// PackageManifestLock
	if bothExtensions(files, packagetypes.PackageManifestLockFilename) {
		return nil, packagetypes.ViolationError{
			Reason: packagetypes.ViolationReasonPackageManifestLockDuplicated,
		}
	}
	if manifestLockBytes, manifestLockPath, manifestLockFound := getFile(
		files, packagetypes.PackageManifestLockFilename); manifestLockFound {
		pkg.ManifestLock, err = manifestLockFromFile(ctx, l.scheme, manifestLockPath, manifestLockBytes)
		if err != nil {
			return nil, err
		}
	}

	// Files
	pkg.Files = files.DeepCopy()
	// remove already processed files
	delete(pkg.Files, packagetypes.PackageManifestFilename+"."+"yml")
	delete(pkg.Files, packagetypes.PackageManifestFilename+"."+"yaml")
	delete(pkg.Files, packagetypes.PackageManifestLockFilename+"."+"yml")
	delete(pkg.Files, packagetypes.PackageManifestLockFilename+"."+"yaml")

	if pkg.Manifest.Spec.Components == nil {
		return pkg, nil
	}

	// Multi-component handling
	if len(componentName) > 0 {
		return nil, packagetypes.ViolationError{
			Reason:    packagetypes.ViolationReasonNestedMultiComponentPkg,
			Component: componentName,
		}
	}

	// Split filesystem by component
	var (
		componentFiles      = map[string]packagetypes.Files{}
		componentPathPrefix = packagetypes.ComponentsFolder + "/"
	)
	for path, file := range pkg.Files {
		if !strings.HasPrefix(path, componentPathPrefix) {
			// non-component file
			continue
		}

		parts := strings.SplitN(path, string(filepath.Separator), 3)
		if len(parts) == 2 {
			return nil, packagetypes.ViolationError{
				Reason: packagetypes.ViolationReasonInvalidFileInComponentsDir,
				Path:   path,
			}
		}
		if len(parts) < 3 {
			return nil, packagetypes.ViolationError{
				Reason: packagetypes.ViolationReasonInvalidComponentPath,
				Path:   path,
			}
		}
		componentName := parts[1] // [0] == "components" [2] == rest

		relPath, err := filepath.Rel(filepath.Join(packagetypes.ComponentsFolder, componentName), path)
		if err != nil {
			return nil, err
		}
		if _, ok := componentFiles[componentName]; !ok {
			componentFiles[componentName] = packagetypes.Files{}
		}
		componentFiles[componentName][relPath] = file

		// delete from root files
		delete(pkg.Files, path)
	}

	for componentName, files := range componentFiles {
		subPkg, err := l.load(ctx, files, componentName)
		if err != nil {
			return nil, err
		}
		pkg.Components = append(pkg.Components, *subPkg)
	}
	return pkg, nil
}

var yamlFileExtensions = []string{"yaml", "yml"}

func bothExtensions(files packagetypes.Files, basename string) bool {
	for _, ext := range yamlFileExtensions {
		path := basename + "." + ext
		if _, ok := files[path]; !ok {
			return false
		}
	}
	return true
}

func getFile(files packagetypes.Files, basename string) (content []byte, path string, ok bool) {
	for _, ext := range yamlFileExtensions {
		path = basename + "." + ext
		content, ok = files[path]
		if ok {
			return
		}
	}
	return
}
