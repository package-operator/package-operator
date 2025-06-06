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
	// +example=['Cluster','Namespaced']
	Scopes []PackageManifestScope `json:"scopes"`
	// Phases correspond to the references to the phases which are going to be the
	// part of the ObjectDeployment/ClusterObjectDeployment.
	Phases []PackageManifestPhase `json:"phases"`
	// Availability Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconciliation of objects in later phases.
	// +optional
	// +example=[]
	AvailabilityProbes []corev1alpha1.ObjectSetProbe `json:"availabilityProbes,omitempty"`
	// Configuration specification.
	Config PackageManifestSpecConfig `json:"config,omitempty"`
	// List of images to be resolved
	Images []PackageManifestImage `json:"images"`
	// Configuration for multi-component packages. If this field is not set it is assumed
	// that the containing package is a single-component package.
	// +optional
	// +example={}
	Components *PackageManifestComponentsConfig `json:"components,omitempty"`
	// Used to filter objects and files based on CEL expressions.
	// +optional
	Filters PackageManifestFilter `json:"filter,omitempty"`
	// Constraints limit what environments a package can be installed into.
	// e.g. can only be installed on OpenShift.
	// +optional
	Constraints []PackageManifestConstraint `json:"constraints,omitempty"`
	// Repository references that are used to validate constraints and resolve dependencies.
	Repositories []PackageManifestRepository `json:"repositories,omitempty"`
	// Dependency references to resolve and use within this package.
	Dependencies []PackageManifestDependency `json:"dependencies,omitempty"`
}

// PackageManifestFilter is used to conditionally render objects based on CEL expressions.
type PackageManifestFilter struct {
	// Reusable CEL expressions. Can be used in 'package-operator.run/condition' annotations.
	// They are evaluated once per package.
	// +optional
	Conditions []PackageManifestNamedCondition `json:"conditions,omitempty"`
	// Adds CEL conditions to file system paths matching a glob pattern.
	// If a single condition matching a file system object's path evaluates to false,
	// the object is ignored.
	Paths []PackageManifestPath `json:"paths,omitempty"`
}

// PackageManifestNamedCondition is a reusable named CEL expression.
// It is injected as a variable into the CEL evaluation environment,
// and its value is set to the result of Expression ("true"/"false").
type PackageManifestNamedCondition struct {
	// A unique name. Must match the CEL identifier pattern: [_a-zA-Z][_a-zA-Z0-9]*
	// +example=isOpenShift
	Name string `json:"name"`
	// A CEL expression with a boolean output type.
	// Has access to the full template context.
	// +example=has(environment.openShift)
	Expression string `json:"expression"`
}

// PackageManifestPath is used to conditionally
// render package objects based on their path.
type PackageManifestPath struct {
	// A file system path glob pattern.
	// Syntax: https://pkg.go.dev/github.com/bmatcuk/doublestar@v1.3.4#Match
	// +example=openshift/v4.15/**
	Glob string `json:"glob"`
	// A CEL expression with a boolean output type.
	// Has access to the full template context and named conditions.
	// +example=cond.isOpenShift && environment.openShift.version.startsWith('4.15')
	Expression string `json:"expression"`
}

// PackageManifestRepository contains information about one package repository
// which could be loaded either from a local file or from a container image.
type PackageManifestRepository struct {
	// References a file in the filesystem to load.
	// +example=../myrepo.yaml
	File string `json:"file,omitempty"`
	// References an image in a container image registry.
	// +example=quay.io/package-operator/my-repo:latest
	Image string `json:"image,omitempty"`
}

// PackageManifestDependency uses a solver to find the latest version package image.
type PackageManifestDependency struct {
	// Resolves the dependency as a image url and digest and commits it to the PackageManifestLock.
	Image *PackageManifestDependencyImage `json:"image,omitempty"`
}

// PackageManifestDependencyImage represents a dependency image found by the solver.
type PackageManifestDependencyImage struct {
	// Name for the dependency.
	// +example=my-pkg
	Name string `json:"name"`
	// Package FQDN <package-name>.<repository name>
	// +example=my-pkg.my-repo
	Package string `json:"package"`
	// Semantic Versioning 2.0.0 version range.
	// +example=>=2.1
	Range string `json:"range"`
}

// PackageManifestConstraint configures environment constraints to block package installation.
type PackageManifestConstraint struct {
	// PackageManifestPlatformVersionConstraint enforces that the platform matches the given version range.
	// This constraint is ignored when running on a different platform.
	// e.g. a PlatformVersionConstraint OpenShift>=4.13.x is ignored when installed on a plain Kubernetes cluster.
	// Use the Platform constraint to enforce running on a specific platform.
	PlatformVersion *PackageManifestPlatformVersionConstraint `json:"platformVersion,omitempty"`
	// Valid platforms that support this package.
	// +example=[Kubernetes]
	Platform []PlatformName `json:"platform,omitempty"`
	// Constraints this package to be only installed once in the Cluster or once in the same Namespace.
	UniqueInScope *PackageManifestUniqueInScopeConstraint `json:"uniqueInScope,omitempty"`
}

// PlatformName holds the name of a specific platform flavor name.
// e.g. Kubernetes, OpenShift.
type PlatformName string

const (
	// Kubernetes platform.
	Kubernetes PlatformName = "Kubernetes"
	// OpenShift platform by Red Hat.
	OpenShift PlatformName = "OpenShift"
)

// PackageManifestPlatformVersionConstraint enforces that the platform matches the given version range.
// This constraint is ignored when running on a different platform.
// e.g. a PlatformVersionConstraint OpenShift>=4.13.x is ignored when installed on a plain Kubernetes cluster.
// Use the Platform constraint to enforce running on a specific platform.
type PackageManifestPlatformVersionConstraint struct {
	// Name of the platform this constraint should apply to.
	// +example=Kubernetes
	Name PlatformName `json:"name"`
	// Semantic Versioning 2.0.0 version range.
	// +example=>=1.20.x
	Range string `json:"range"`
}

// PackageManifestUniqueInScopeConstraint constraints this package
// to be only installed once in the Cluster or once in the same Namespace.
type PackageManifestUniqueInScopeConstraint struct{}

// PackageManifestComponentsConfig configures components of a package.
type PackageManifestComponentsConfig struct{}

// PackageManifestSpecConfig configutes a package manifest.
type PackageManifestSpecConfig struct {
	// OpenAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	// +example={type: object,properties: {testProp: {type: string}}}
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`
}

// PackageManifestPhase defines a package phase.
type PackageManifestPhase struct {
	// Name of the reconcile phase. Must be unique within a PackageManifest
	// +example=deploy
	Name string `json:"name"`
	// If non empty, phase reconciliation is delegated to another controller.
	// If set to the string "default" the built-in controller reconciling the object.
	// If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects.
	// +example=hosted-cluster
	Class string `json:"class,omitempty"`
}

// PackageManifestImage specifies an image tag to be resolved.
type PackageManifestImage struct {
	// Image name to be use to reference it in the templates
	// +example=test-stub
	Name string `json:"name"`
	// Image identifier (REPOSITORY[:TAG])
	// +example=quay.io/package-operator/test-stub:v1.11.0
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
	// +example=v1.29.5
	KubernetesVersion string `json:"kubernetesVersion"`
	//nolint:lll
	// OpenAPI schema locations for kubeconform
	// defaults to:
	// - https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{ .NormalizedKubernetesVersion }}-standalone{{ .StrictSuffix }}/{{ .ResourceKind }}{{ .KindSuffix }}.json
	// - https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json
	// +example=['https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{.NormalizedKubernetesVersion}}-standalone{{.StrictSuffix}}/{{.ResourceKind}}{{.KindSuffix}}.json']
	SchemaLocations []string `json:"schemaLocations,omitempty"`
}

// TemplateContext is available within the package templating process.
type TemplateContext struct {
	// Package object.
	// +example={image: quay.io/package-operator/test-stub-package:v1.11.0, metadata: {name: test}}
	Package TemplateContextPackage `json:"package"`
	// Configuration as presented via the (Cluster)Package API after admission.
	// +example={testProp: Hans}
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
	// +example=v1.29.5
	Version string `json:"version"`
}

// PackageEnvironmentOpenShift configures openshift environments.
type PackageEnvironmentOpenShift struct {
	// OpenShift server version.
	// +example=v4.13.2
	Version string `json:"version"`
	// ManagedOpenShift environment information. This section is only set when a managed OpenShift cluster is detected.
	// This includes Red Hat OpenShift Dedicated, Red Hat OpenShift Service on AWS (ROSA) and
	// Azure Red Hat OpenShift (ARO) and their Hosted Control Plane variants.
	Managed *PackageEnvironmentManagedOpenShift `json:"managed,omitempty"`
}

// PackageEnvironmentManagedOpenShift describes managed OpenShift environments.
type PackageEnvironmentManagedOpenShift struct {
	// Data key-value pairs describing details of the Managed OpenShift environment.
	// +example={test: test}
	Data map[string]string `json:"data"`
}

// PackageEnvironmentProxy configures proxy environments.
// On OpenShift, this config is taken from the cluster Proxy object.
// https://docs.openshift.com/container-platform/4.13/networking/enable-cluster-wide-proxy.html
type PackageEnvironmentProxy struct {
	// HTTP_PROXY
	// +example=http://proxy_server_address:port
	HTTPProxy string `json:"httpProxy,omitempty"`
	// HTTPS_PROXY
	// +example=https://proxy_server_address:port
	HTTPSProxy string `json:"httpsProxy,omitempty"`
	// NO_PROXY
	// +example=".example.com,.local,localhost"
	NoProxy string `json:"noProxy,omitempty"`
}

// PackageEnvironmentHyperShift contains HyperShift specific information.
// Only available when installed alongside HyperShift.
// https://github.com/openshift/hypershift
type PackageEnvironmentHyperShift struct {
	// Contains HyperShift HostedCluster specific information.
	// This information is only available when installed alongside HyperShift within a HostedCluster Namespace.
	// https://github.com/openshift/hypershift
	// +example={hostedClusterNamespace: clusters-banana, metadata: {name: banana, namespace: clusters}}
	HostedCluster *PackageEnvironmentHyperShiftHostedCluster `json:"hostedCluster"`
}

// PackageEnvironmentHyperShiftHostedCluster contains HyperShift HostedCluster specific information.
// This information is only available when installed alongside HyperShift within a HostedCluster Namespace.
// https://github.com/openshift/hypershift
type PackageEnvironmentHyperShiftHostedCluster struct {
	TemplateContextObjectMeta `json:"metadata"`

	// The control-plane namespace of this hosted cluster.
	// Note: This should actually be named HostedControlPlaneNamespace, but renaming would change our template API.
	HostedClusterNamespace string `json:"hostedClusterNamespace"`

	// NodeSelector when specified in HostedCluster.spec.nodeSelector, is propagated to all control plane Deployments
	// and Stateful sets running management side.
	//
	// Note: Upstream docs of this field specify that changing it will re-deploy
	// existing control-plane workloads. This is not something that PKO currently supports.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
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
