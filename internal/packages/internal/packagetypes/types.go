package packagetypes

import (
	"maps"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/apis/manifests"
)

// Package has passed basic schema/structure admission.
// Exact output still depends on configuration and
// the install environment.
type Package struct {
	Manifest     *manifests.PackageManifest
	ManifestLock *manifests.PackageManifestLock
	Files        Files
	Components   []Package
}

// Returns a deep copy of the RawPackage map.
func (p *Package) DeepCopy() *Package {
	cp := &Package{
		Manifest: p.Manifest.DeepCopy(),
		Files:    p.Files.DeepCopy(),
	}
	if p.ManifestLock != nil {
		cp.ManifestLock = p.ManifestLock.DeepCopy()
	}
	for _, comp := range p.Components {
		cp.Components = append(cp.Components, *comp.DeepCopy())
	}
	return cp
}

// PackageInstance is the concrete instance of a package after rendering
// templates from configuration and environment information.
type PackageInstance struct {
	Manifest     *manifests.PackageManifest
	ManifestLock *manifests.PackageManifestLock
	Objects      []unstructured.Unstructured
}

// PackageRenderContext contains all data that is needed to render a Package into a PackageInstance.
type PackageRenderContext struct {
	Package     manifests.TemplateContextPackage `json:"package"`
	Config      map[string]any                   `json:"config"`
	Images      map[string]string                `json:"images"`
	Environment manifests.PackageEnvironment     `json:"environment"`
}

// RawPackage right after import.
// No validation has been performed yet.
type RawPackage struct {
	// Labels added by the transport format.
	// In most cases these will be OCI labels.
	Labels map[string]string
	Files  Files
}

// Returns a deep copy of the RawPackage map.
func (rp *RawPackage) DeepCopy() *RawPackage {
	return &RawPackage{
		Labels: maps.Clone(rp.Labels),
		Files:  rp.Files.DeepCopy(),
	}
}

// Files is an in-memory representation of the package FileSystem.
// It maps file paths to their contents.
type Files map[string][]byte

// Returns a deep copy of the files map.
func (f Files) DeepCopy() Files {
	newF := Files{}
	for k, v := range f {
		newV := make([]byte, len(v))
		copy(newV, v)
		newF[k] = newV
	}
	return newF
}

var (
	// PackageManifestGroupKind is the kubernetes schema group kind of a PackageManifest.
	PackageManifestGroupKind = schema.GroupKind{Group: manifestsv1alpha1.GroupVersion.Group, Kind: "PackageManifest"}
	// PackageManifestLockGroupKind is the kubernetes schema group kind of a PackageManifestLock.
	PackageManifestLockGroupKind = schema.GroupKind{
		Group: manifestsv1alpha1.GroupVersion.Group, Kind: "PackageManifestLock",
	}
)
