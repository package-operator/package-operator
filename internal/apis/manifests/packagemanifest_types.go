package manifests

import (
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

const (
	PackagePhaseAnnotation          = manifestsv1alpha1.PackagePhaseAnnotation
	PackageConditionMapAnnotation   = manifestsv1alpha1.PackageConditionMapAnnotation
	PackageExternalObjectAnnotation = manifestsv1alpha1.PackageExternalObjectAnnotation
)

const (
	PackageLabel                 = manifestsv1alpha1.PackageLabel
	PackageSourceImageAnnotation = manifestsv1alpha1.PackageSourceImageAnnotation
	PackageConfigAnnotation      = manifestsv1alpha1.PackageConfigAnnotation
	PackageInstanceLabel         = manifestsv1alpha1.PackageInstanceLabel
)

// +kubebuilder:object:root=true
type PackageManifest struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec PackageManifestSpec
	Test PackageManifestTest
}

// PackageManifestScope declares the available scopes to install this package in.
type PackageManifestScope string

const (
	// Cluster scope allows the package to be installed for the whole cluster.
	// The package needs to default installation namespaces and create them.
	PackageManifestScopeCluster PackageManifestScope = "Cluster"
	// Namespace scope allows the package to be installed for specific namespaces.
	PackageManifestScopeNamespaced PackageManifestScope = "Namespaced"
)

// PackageManifestSpec represents the spec of the packagemanifest containing the details about phases and availability probes.
type PackageManifestSpec struct {
	// Scopes declare the available installation scopes for the package.
	// Either Cluster, Namespaced, or both.
	Scopes []PackageManifestScope
	// Phases correspond to the references to the phases which are going to be the part of the ObjectDeployment/ClusterObjectDeployment.
	Phases []PackageManifestPhase
	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	// +optional
	AvailabilityProbes []corev1alpha1.ObjectSetProbe
	// Configuration specification.
	Config PackageManifestSpecConfig
	// List of images to be resolved
	Images []PackageManifestImage
	// Configuration for multi-component packages. If this field is not set it is assumed that the containing package is a single-component package.
	// +optional
	Components *PackageManifestComponentsConfig
	// Constraints limit what environments a package can be installed into.
	// e.g. can only be installed on OpenShift.
	// +optional
	Constraints []PackageManifestConstraint
	// Repository references that are used to validate constraints and resolve dependencies.
	Repositories []PackageManifestRepository
	// Dependency references to resolve and use within this package.
	Dependencies []PackageManifestDependency
}

type PackageManifestRepository struct {
	// References a file in the filesystem to load.
	// +example=../myrepo.yaml
	File string
	// References an image in a container image registry.
	// +example=quay.io/package-operator/my-repo:latest
	Image string
}

// Uses a solver to find the latest version package image.
type PackageManifestDependency struct {
	// Resolves the dependency as a image url and digest and commits it to the PackageManifestLock.
	Image *PackageManifestDependencyImage
}

type PackageManifestDependencyImage struct {
	// Name for the dependency.
	// +example=my-pkg
	Name string
	// Package FQDN <package-name>.<repository name>
	// +example=my-pkg.my-repo
	Package string
	// Semantic Versioning 2.0.0 version range.
	// +example=>=2.1
	Range string
}

// PackageManifestConstraint configures environment constraints to block package installation.
type PackageManifestConstraint struct {
	// PackageManifestPlatformVersionConstraint enforces that the platform matches the given version range.
	// This constraint is ignored when running on a different platform.
	// e.g. a PlatformVersionConstraint OpenShift>=4.13.x is ignored when installed on a plain Kubernetes cluster.
	// Use the Platform constraint to enforce running on a specific platform.
	PlatformVersion *PackageManifestPlatformVersionConstraint
	// Valid platforms that support this package.
	// +example=[Kubernetes]
	Platform []PlatformName
	// Constraints this package to be only installed once in the Cluster or once in the same Namespace.
	UniqueInScope *PackageManifestUniqueInScopeConstraint
}
type PlatformName string

const (
	Kubernetes PlatformName = "Kubernetes"
	OpenShift  PlatformName = "OpenShift"
)

type PackageManifestPlatformVersionConstraint struct {
	// Name of the platform this constraint should apply to.
	Name PlatformName
	// Semantic Versioning 2.0.0 version range.
	Range string
}

type PackageManifestUniqueInScopeConstraint struct{}

type PackageManifestComponentsConfig struct{}

type PackageManifestSpecConfig struct {
	OpenAPIV3Schema *apiextensions.JSONSchemaProps
}

type PackageManifestPhase struct {
	// Name of the reconcile phase. Must be unique within a PackageManifest
	Name string
	// If non empty, phase reconciliation is delegated to another controller.
	// If set to the string "default" the built-in controller reconciling the object.
	// If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects.
	Class string
}

// PackageManifestImage specifies an image tag to be resolved.
type PackageManifestImage struct {
	// Image name to be use to reference it in the templates
	Name string
	// Image identifier (REPOSITORY[:TAG])
	Image string
}

// PackageManifestTest configures test cases.
type PackageManifestTest struct {
	// Template testing configuration.
	Template    []PackageManifestTestCaseTemplate
	Kubeconform *PackageManifestTestKubeconform
}

// PackageManifestTestCaseTemplate template testing configuration.
type PackageManifestTestCaseTemplate struct {
	// Name describing the test case.
	Name string
	// Template data to use in the test case.
	Context TemplateContext
}

type PackageManifestTestKubeconform struct {
	// Kubernetes version to use schemas from.
	KubernetesVersion string
	// OpenAPI schema locations for kubeconform
	// defaults to:
	// - https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{ .NormalizedKubernetesVersion }}-standalone{{ .StrictSuffix }}/{{ .ResourceKind }}{{ .KindSuffix }}.json
	// - https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json
	SchemaLocations []string
}

// TemplateContext is available within the package templating process.
type TemplateContext struct {
	// Package object.
	Package TemplateContextPackage `json:"package"`
	// Configuration as presented via the (Cluster)Package API after admission.
	Config *runtime.RawExtension `json:"config,omitempty"`
	// Environment specific information.
	Environment PackageEnvironment `json:"environment"`
}

// PackageEnvironment information.
type PackageEnvironment struct {
	// Kubernetes environment information.
	Kubernetes PackageEnvironmentKubernetes `json:"kubernetes"`
	// OpenShift environment information.
	OpenShift *PackageEnvironmentOpenShift `json:"openShift,omitempty"`
	// Proxy configuration.
	Proxy *PackageEnvironmentProxy `json:"proxy,omitempty"`
}

type PackageEnvironmentKubernetes struct {
	// Kubernetes server version.
	Version string `json:"version"`
}

type PackageEnvironmentOpenShift struct {
	// OpenShift server version.
	Version string `json:"version"`
}

// Environment proxy settings.
// On OpenShift, this config is taken from the cluster Proxy object.
// https://docs.openshift.com/container-platform/4.13/networking/enable-cluster-wide-proxy.html
type PackageEnvironmentProxy struct {
	// HTTP_PROXY
	HTTPProxy string `json:"httpProxy,omitempty"`
	// HTTPS_PROXY
	HTTPSProxy string `json:"httpsProxy,omitempty"`
	// NO_PROXY
	NoProxy string `json:"noProxy,omitempty"`
}

// TemplateContextPackage represents the (Cluster)Package object requesting this package content.
type TemplateContextPackage struct {
	TemplateContextObjectMeta `json:"metadata"`
}

// TemplateContextObjectMeta represents a simplified version of metav1.ObjectMeta for use in templates.
type TemplateContextObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func init() { register(&PackageManifest{}) }
