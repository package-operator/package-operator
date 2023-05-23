package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	// Package Phase annotation to assign objects to a phase.
	PackagePhaseAnnotation = "package-operator.run/phase"
	// Package ConditionMap annotation specifies object conditions to map back into Package Operator APIs.
	// Example: Available => my-own-prefix/Available.
	PackageConditionMapAnnotation = "package-operator.run/condition-map"
	// Package ExternalObject annotation, when set to "True", indicates
	// that the referenced object should only be observed during a phase
	// rather than reconciled.
	PackageExternalObjectAnnotation = "package-operator.run/external"
)

const (
	// PackageLabel contains the name of the Package from the PackageManifest.
	PackageLabel = "package-operator.run/package"
	// PackageSourceImageAnnotation references the package container image originating this object.
	PackageSourceImageAnnotation = "package-operator.run/package-source-image"
	// PackageConfigAnnotation contains the configuration for this object.
	PackageConfigAnnotation = "package-operator.run/package-config"
	// PackageInstanceLabel contains the name of the Package instance.
	PackageInstanceLabel = "package-operator.run/instance"
)

// +kubebuilder:object:root=true
type PackageManifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageManifestSpec `json:"spec,omitempty"`
	Test PackageManifestTest `json:"test,omitempty"`
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
	Scopes []PackageManifestScope `json:"scopes"`
	// Phases correspond to the references to the phases which are going to be the part of the ObjectDeployment/ClusterObjectDeployment.
	Phases []PackageManifestPhase `json:"phases"`
	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	// +optional
	AvailabilityProbes []corev1alpha1.ObjectSetProbe `json:"availabilityProbes,omitempty"`
	// Configuration specification.
	Config PackageManifestSpecConfig `json:"config,omitempty"`
	// List of images to be resolved
	Images []PackageManifestImage `json:"images"`
}

type PackageManifestSpecConfig struct {
	// OpenAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`
}

type PackageManifestPhase struct {
	// Name of the reconcile phase. Must be unique within a PackageManifest
	Name string `json:"name"`
	// If non empty, phase reconciliation is delegated to another controller.
	// If set to the string "default" the built-in controller reconciling the object.
	// If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects.
	Class string `json:"class,omitempty"`
}

// PackageManifestImage specifies an image tag to be resolved
type PackageManifestImage struct {
	// Image name to be use to reference it in the templates
	Name string `json:"name"`
	// Image identifier (REPOSITORY[:TAG])
	Image string `json:"image"`
}

// PackageManifestTest configures test cases.
type PackageManifestTest struct {
	// Template testing configuration.
	Template []PackageManifestTestCaseTemplate `json:"template,omitempty"`
}

// PackageManifestTestCaseTemplate template testing configuration.
type PackageManifestTestCaseTemplate struct {
	// Name describing the test case.
	Name string `json:"name"`
	// Template data to use in the test case.
	Context TemplateContext `json:"context,omitempty"`
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
