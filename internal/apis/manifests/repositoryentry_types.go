package manifests

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
type RepositoryEntry struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Data RepositoryEntryData
}

type RepositoryEntryData struct {
	// OCI host/repository and name.
	// e.g. quay.io/xxx/xxx
	Image string
	// Image digest uniquely identifying this image.
	Digest string
	// Semver V2 versions that are assigned to the package.
	Versions []string
	// Constraints of the package.
	Constraints []PackageManifestConstraint
	// Name of the package.
	Name string
}

func init() { register(&RepositoryEntry{}) }
