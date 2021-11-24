// Common structs objects
package v1alpha1

// References a secret on the cluster.
type ClusterSecretReference struct {
	// Name of the secret object.
	Name string `json:"name"`
	// Namespace of the secret object.
	Namespace string `json:"namespace"`
}
