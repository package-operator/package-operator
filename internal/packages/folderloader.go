// Package packages provides the facilities to read package directory.
// And to compile these files into an (Cluster)ObjectDeployment.
package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

// PackageManifest file names to probe for.
var packageManifestFileNames = []string{
	"manifest.yaml",
	"manifest.yml",
}

var packageManifestGroupKind = schema.GroupKind{
	Group: manifestsv1alpha1.GroupVersion.Group,
	Kind:  "PackageManifest",
}

// FolderLoader reads a package as files in the filesystem.
// Parsing them according to their packaging specification.
type FolderLoader struct {
	scheme *runtime.Scheme
}

func NewFolderLoader(scheme *runtime.Scheme) *FolderLoader {
	return &FolderLoader{
		scheme: scheme,
	}
}

// Template Context provided when executing file templates.
type FolderLoaderTemplateContext struct {
	Package PackageTemplateContext
}

type PackageTemplateContext struct {
	metav1.ObjectMeta
}

// Result of a loading operation.
type FolderLoaderResult struct {
	Annotations, Labels map[string]string
	TemplateSpec        corev1alpha1.ObjectSetTemplateSpec
	Manifest            *manifestsv1alpha1.PackageManifest
}

func (l *FolderLoader) Load(
	ctx context.Context, rootPath string,
	templateContext FolderLoaderTemplateContext,
) (res FolderLoaderResult, err error) {
	res.Manifest, err = l.FindManifest(ctx, rootPath)
	if err != nil {
		return res, err
	}

	loadContext := newFolderLoadOperation(logr.FromContextOrDiscard(ctx), rootPath, res.Manifest, templateContext)
	if err := loadContext.Load(); err != nil {
		return res, err
	}

	res.Annotations = map[string]string{}
	res.Labels = commonLabels(res.Manifest, templateContext.Package.Name)
	res.TemplateSpec.AvailabilityProbes = res.Manifest.Spec.AvailabilityProbes

	for _, phase := range res.Manifest.Spec.Phases {
		phase := corev1alpha1.ObjectSetTemplatePhase{
			Name:  phase.Name,
			Class: phase.Class,
		}
		unstructuredObjects := loadContext.objectsByPhase[phase.Name]
		for _, obj := range unstructuredObjects {
			phase.Objects = append(phase.Objects, corev1alpha1.ObjectSetObject{
				Object: obj,
			})
		}

		if len(phase.Objects) == 0 {
			// empty phases may happen due to templating for scope or topology restrictions.
			continue
		}

		res.TemplateSpec.Phases = append(res.TemplateSpec.Phases, phase)
	}

	return res, nil
}

// Tries to find the PackageManifest at multiple predetermined locations.
func (l *FolderLoader) FindManifest(ctx context.Context, rootPath string) (
	*manifestsv1alpha1.PackageManifest, error,
) {
	for _, packageManifestFileName := range packageManifestFileNames {
		manifest, err := l.LoadPackageManifest(ctx, path.Join(rootPath, packageManifestFileName))
		if err == nil {
			return manifest, nil
		}

		var e *PackageManifestNotFoundError
		if errors.As(err, &e) {
			// don't count not found yet, until we tried all file names.
			continue
		}
		// some other error.
		return nil, err
	}

	return nil, &PackageManifestNotFoundError{}
}

// Loads a PackageManifest at the given path.
// Converts whatever PackageManifest version it finds into an v1alpha1 PackageManifest.
func (l *FolderLoader) LoadPackageManifest(ctx context.Context, filePath string) (
	*manifestsv1alpha1.PackageManifest, error,
) {
	b, err := os.ReadFile(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, &PackageManifestNotFoundError{}
	}
	if err != nil {
		return nil, err
	}

	var manifestType metav1.TypeMeta
	if err := yaml.Unmarshal(b, &manifestType); err != nil {
		return nil, &PackageManifestInvalidError{
			Reason: fmt.Sprintf("yaml load: %v", err),
		}
	}

	gvk := manifestType.GroupVersionKind()

	if gvk.GroupKind() != packageManifestGroupKind {
		return nil, &PackageManifestInvalidError{
			Reason: fmt.Sprintf("GroupKind must be %s, is: %s", packageManifestGroupKind, gvk.GroupKind()),
		}
	}

	if !l.scheme.Recognizes(gvk) {
		// GroupKind is ok, so the version is not recognized.
		// Either the Package we are trying is very old and support was dropped or
		// Package is build for a newer PKO version.
		groupVersions := l.scheme.VersionsForGroupKind(gvk.GroupKind())
		versions := make([]string, len(groupVersions))
		for i := range groupVersions {
			versions[i] = groupVersions[i].Version
		}

		return nil, &PackageManifestInvalidError{
			Reason: fmt.Sprintf("unknown version %s, supported versions: %s", gvk.Version, strings.Join(versions, ", ")),
		}
	}

	anyVersionPackageManifest, err := l.scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, anyVersionPackageManifest); err != nil {
		return nil, &PackageManifestInvalidError{
			Reason: err.Error(),
		}
	}

	// Default fields in PackageManifest
	l.scheme.Default(anyVersionPackageManifest)

	// Whatever PackageManifest version we have loaded,
	// we have to convert it to a common/hub version to use throughout the code base:
	manifest := &manifestsv1alpha1.PackageManifest{}
	if err := l.scheme.Convert(anyVersionPackageManifest, manifest, nil); err != nil {
		return nil, &PackageManifestInvalidError{
			Reason: fmt.Sprintf("converting to hub version: %v", err),
		}
	}

	if err := manifest.Validate(); err != nil {
		return nil, &PackageManifestInvalidError{
			Err: err,
		}
	}
	return manifest, nil
}

func commonLabels(manifest *manifestsv1alpha1.PackageManifest, packageName string) map[string]string {
	return map[string]string{
		manifestsv1alpha1.PackageLabel:         manifest.Name,
		manifestsv1alpha1.PackageInstanceLabel: packageName,
	}
}
