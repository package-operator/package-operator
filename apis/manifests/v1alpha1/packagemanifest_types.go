package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	// PackagePhaseAnnotation annotation to assign objects to a phase.
	PackagePhaseAnnotation = "package-operator.run/phase"
	// PackageConditionMapAnnotation specifies object conditions to map back
	// into Package Operator APIs.
	// Example: Available => my-own-prefix/Available.
	PackageConditionMapAnnotation = "package-operator.run/condition-map"
	// PackageExternalObjectAnnotation when set to "True", indicates
	// that the referenced object should only be observed during a phase
	// rather than reconciled.
	PackageExternalObjectAnnotation = "package-operator.run/external"
	// PackageCELConditionAnnotation contains a CEL expression
	// evaluating to a boolean value which determines whether the object is created.
	PackageCELConditionAnnotation = "package-operator.run/condition"
	// PackageCollisionProtectionAnnotation prevents Package Operator from working
	// on objects already under management by a different operator.
	PackageCollisionProtectionAnnotation = "package-operator.run/collision-protection"
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

// PackageManifest defines the manifest of a package.
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
	// PackageManifestScopeCluster scope allows the package to be installed for the whole cluster.
	// The package needs to default installation namespaces and create them.
	PackageManifestScopeCluster PackageManifestScope = "Cluster"
	// PackageManifestScopeNamespaced scope allows the package to be installed for specific namespaces.
	PackageManifestScopeNamespaced PackageManifestScope = "Namespaced"
)

// PackageManifestSpec represents the spec of the packagemanifest containing the
// details about phases and availability probes.
type PackageManifestSpec struct {
	// Scopes declare the available installation scopes for the package.
	// Either Cluster, Namespaced, or both.
	Scopes []PackageManifestScope `json:"scopes"`
	// Phases correspond to the references to the phases which are going to be the
	// part of the ObjectDeployment/ClusterObjectDeployment.
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
	// Configuration for multi-component packages. If this field is not set it is assumed
	// that the containing package is a single-component package.
	// +optional
	Components *PackageManifestComponentsConfig `json:"components,omitempty"`
	// Used to conditionally render objects based on CEL expressions.
	// +optional
	ConditionalFiltering PackageManifestConditionalFiltering `json:"conditionalFiltering,omitempty"`
}

// PackageManifestConditionalFiltering is used to conditionally render objects based on CEL expressions.
type PackageManifestConditionalFiltering struct {
	// Reusable CEL expressions. Can be used in 'package-operator.run/condition' annotations.
	// They are evaluated once per package.
	// +optional
	NamedConditions []PackageManifestNamedCondition `json:"namedConditions,omitempty"`
	// Adds CEL conditions to file system paths matching a glob pattern.
	// If a single condition matching a file system object's path evaluates to false,
	// the object is ignored.
	ConditionalPaths []PackageManifestConditionalPath `json:"conditionalPaths,omitempty"`
}

// PackageManifestNamedCondition is a reusable named CEL expression.
// It is injected as a variable into the CEL evaluation environment,
// and its value is set to the result of Expression ("true"/"false").
type PackageManifestNamedCondition struct {
	// A unique name. Must match the CEL identifier pattern: [_a-zA-Z][_a-zA-Z0-9]*
	Name string `json:"name"`
	// A CEL expression with a boolean output type.
	// Has access to the full template context.
	Expression string `json:"expression"`
}

// PackageManifestConditionalPath is used to conditionally
// render package objects based on their path.
type PackageManifestConditionalPath struct {
	// A file system path glob pattern.
	// Syntax: https://pkg.go.dev/github.com/bmatcuk/doublestar@v1.3.4#Match
	Glob string `json:"glob"`
	// A CEL expression with a boolean output type.
	// Has access to the full template context and named conditions.
	Expression string `json:"expression"`
}

// PackageManifestComponentsConfig configures components of a package.
type PackageManifestComponentsConfig struct{}

// PackageManifestSpecConfig configutes a package manifest.
type PackageManifestSpecConfig struct {
	// OpenAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`
}

// PackageManifestPhase defines a package phase.
type PackageManifestPhase struct {
	// Name of the reconcile phase. Must be unique within a PackageManifest
	Name string `json:"name"`
	// If non empty, phase reconciliation is delegated to another controller.
	// If set to the string "default" the built-in controller reconciling the object.
	// If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects.
	Class string `json:"class,omitempty"`
}

// PackageManifestImage specifies an image tag to be resolved.
type PackageManifestImage struct {
	// Image name to be use to reference it in the templates
	Name string `json:"name"`
	// Image identifier (REPOSITORY[:TAG])
	Image string `json:"image"`
}

// PackageManifestTest configures test cases.
type PackageManifestTest struct {
	// Template testing configuration.
	Template    []PackageManifestTestCaseTemplate `json:"template,omitempty"`
	Kubeconform *PackageManifestTestKubeconform   `json:"kubeconform,omitempty"`
}

// PackageManifestTestCaseTemplate template testing configuration.
type PackageManifestTestCaseTemplate struct {
	// Name describing the test case.
	Name string `json:"name"`
	// Template data to use in the test case.
	Context TemplateContext `json:"context,omitempty"`
}

// PackageManifestTestKubeconform configures kubeconform testing.
type PackageManifestTestKubeconform struct {
	// Kubernetes version to use schemas from.
	KubernetesVersion string `json:"kubernetesVersion"`
	//nolint:lll
	// OpenAPI schema locations for kubeconform
	// defaults to:
	// - https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{ .NormalizedKubernetesVersion }}-standalone{{ .StrictSuffix }}/{{ .ResourceKind }}{{ .KindSuffix }}.json
	// - https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json
	SchemaLocations []string `json:"schemaLocations,omitempty"`
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
	// Kubernetes environment information. This section is always set.
	Kubernetes PackageEnvironmentKubernetes `json:"kubernetes"`
	// OpenShift environment information. This section is only set when OpenShift is detected.
	OpenShift *PackageEnvironmentOpenShift `json:"openShift,omitempty"`
	// Proxy configuration. Only available on OpenShift when the cluster-wide Proxy is enabled.
	// https://docs.openshift.com/container-platform/latest/networking/enable-cluster-wide-proxy.html
	Proxy *PackageEnvironmentProxy `json:"proxy,omitempty"`
	// HyperShift specific information. Only available when installed alongside HyperShift.
	// https://github.com/openshift/hypershift
	HyperShift *PackageEnvironmentHyperShift `json:"hyperShift,omitempty"`
}

// PackageEnvironmentKubernetes configures kubernetes environments.
type PackageEnvironmentKubernetes struct {
	// Kubernetes server version.
	Version string `json:"version"`
}

// PackageEnvironmentOpenShift configures openshift environments.
type PackageEnvironmentOpenShift struct {
	// OpenShift server version.
	Version string `json:"version"`
}

// PackageEnvironmentProxy configures proxy environments.
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

// PackageEnvironmentHyperShift contains HyperShift specific information.
// Only available when installed alongside HyperShift.
// https://github.com/openshift/hypershift
type PackageEnvironmentHyperShift struct {
	// Contains HyperShift HostedCluster specific information.
	// This information is only available when installed alongside HyperShift within a HostedCluster Namespace.
	// https://github.com/openshift/hypershift
	HostedCluster *PackageEnvironmentHyperShiftHostedCluster `json:"hostedCluster"`
}

// PackageEnvironmentHyperShiftHostedCluster contains HyperShift HostedCluster specific information.
// This information is only available when installed alongside HyperShift within a HostedCluster Namespace.
// https://github.com/openshift/hypershift
type PackageEnvironmentHyperShiftHostedCluster struct {
	TemplateContextObjectMeta `json:"metadata"`
	// Namespace of HostedCluster components belonging to this HostedCluster object.
	HostedClusterNamespace string `json:"hostedClusterNamespace"`
}

// TemplateContextPackage represents the (Cluster)Package object requesting this package content.
type TemplateContextPackage struct {
	TemplateContextObjectMeta `json:"metadata"`
	// Image as presented via the (Cluster)Package API after admission.
	Image string `json:"image"`
}

// TemplateContextObjectMeta represents a simplified version of metav1.ObjectMeta for use in templates.
type TemplateContextObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func init() { register(&PackageManifest{}) }
