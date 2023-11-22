package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
type RepositoryEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Data RepositoryEntryData `json:"data"`
}

type RepositoryEntryData struct {
	// OCI host/repository and name.
	// e.g. quay.io/xxx/xxx
	Image string `json:"image"`
	// Image digest uniquely identifying this image.
	Digest string `json:"digest"`
	// Semver V2 versions that are assigned to the package.
	Versions []string `json:"versions"`
	// Constraints of the package.
	Constraints []PackageManifestConstraint `json:"constraints,omitempty"`
	// Name of the package.
	Name string `json:"name,omitempty"`
}

func init() { register(&RepositoryEntry{}) }
