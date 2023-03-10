package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
type RepositoryPackageManifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Repository *PackageManifestRepository    `json:"repository,omitempty"`
	Spec       RepositoryPackageManifestSpec `json:"spec,omitempty"`
}

type RepositoryPackageManifestSpec struct {
	// Scopes declare the available installation scopes for the package.
	// Either Cluster, Namespaced, or both.
	Scopes []PackageManifestScope `json:"scopes"`

	// Release channels this Package is available in.
	Channels []string `json:"channels,omitempty"`

	// PackageImage URL.
	PackageImage string `json:"packageImage"`

	// Resolved Images that are deployed as part of this package.
	ResolvedImages []PackageManifestLockImage `json:"resolvedImages,omitempty"`
}

func init() { register(&RepositoryPackageManifest{}) }
