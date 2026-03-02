package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RepositoryStatus defines the observed state of a Repository.
type RepositoryStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// When evaluating object state in code, use .Conditions instead.
	Phase RepositoryStatusPhase `json:"phase,omitempty"`
	// Hash of image + config that was successfully unpacked.
	UnpackedHash string `json:"unpackedHash,omitempty"`
}

// Repository condition types.
const (
	// A Repository "Available" condition tracks the availability of the underlying ObjectDeployment objects.
	// When the Repository is reporting "Available" = True, it's expected that whatever the Repository installs
	// is up and operational. Repository "Availability" may change multiple times during it's lifecycle.
	RepositoryAvailable = "Available"
	// Unpacked tracks the completion or failure of the image unpack operation.
	RepositoryUnpacked = "Unpacked"
	// Invalid condition tracks unrecoverable validation and loading issues of the Repository.
	// A repository might be invalid because of multiple reasons:
	// - Does not support the right scope -> Namespaced vs. Cluster
	// - Malformed Yaml.
	RepositoryInvalid = "Invalid"
)

// RepositoryStatusPhase defines a status phase of a Repository.
type RepositoryStatusPhase string

// Well-known Repository Phases for printing a Status in kubectl,
// see deprecation notice in RepositoryStatus for details.
const (
	RepositoryPhasePending   RepositoryStatusPhase = "Pending"
	RepositoryPhaseAvailable RepositoryStatusPhase = "Available"
	RepositoryPhaseUnpacking RepositoryStatusPhase = "Unpacking"
	RepositoryPhaseNotReady  RepositoryStatusPhase = "NotReady"
	RepositoryPhaseInvalid   RepositoryStatusPhase = "Invalid"
)

// RepositorySpec specifies a repository.
type RepositorySpec struct {
	// the image containing the contents of the repository
	// +kubebuilder:validation:Required
	Image string `json:"image"`
}
