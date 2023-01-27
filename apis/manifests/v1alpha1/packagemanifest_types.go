package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	// Package Phase annotation to assign objects to a phase.
	PackagePhaseAnnotation = "package-operator.run/phase"
)

const (
	// PackageLabel contains the name of the Package from the PackageManifest.
	PackageLabel = "package-operator.run/package"
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
	AvailabilityProbes []corev1alpha1.ObjectSetProbe `json:"availabilityProbes"`
	// Digest Replacements define in which objects to replace image tags with digests.
	DigestReplacements DigestReplacements `json:"digestReplacements"`
}

type PackageManifestPhase struct {
	// Name of the reconcile phase. Must be unique within a PackageManifest
	Name string `json:"name"`
	// If non empty, phase reconciliation is delegated to another controller.
	// If set to the string "default" the built-in controller reconciling the object.
	// If set to any other string, an out-of-tree controller needs to be present to handle ObjectSetPhase objects.
	Class string `json:"class,omitempty"`
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
	Package TemplateContextPackage `json:"package"`
}

// TemplateContextPackage represents the (Cluster)Package object requesting this package content.
type TemplateContextPackage struct {
	TemplateContextObjectMeta `json:"metadata"`
}

// TemplateContextObjectMeta represents a simplified version of metav1.ObjectMeta for use in templates.
type TemplateContextObjectMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type DigestReplacements []DigestReplacementSelector

// DigestReplacementSelector selects a subset of objects to replace image tags with digests.
type DigestReplacementSelector struct {
	// Kind and API Group of the object to probe.
	Kind *DigestReplacementKindSpec `json:"kind"`
}

// Kind package probe parameters.
// DigestReplacementKindSpec selects objects based on Kind and API version.
type DigestReplacementKindSpec struct {
	// Object Group to apply a probe to.
	// +example=apps/v1
	ApiVersion string `json:"apiVersion"`
	// Object Kind to apply a probe to.
	// +example=Deployment
	Kind string `json:"kind"`
}

// Validates the PackageManifest and returns an aggregated list of all errors.
func (m *PackageManifest) Validate() error {
	var allErrs field.ErrorList

	if len(m.Name) == 0 {
		allErrs = append(allErrs,
			field.Required(field.NewPath("metadata").Child("name"), ""))
	}

	spec := field.NewPath("spec")
	if len(m.Spec.Scopes) == 0 {
		allErrs = append(allErrs,
			field.Required(spec.Child("scopes"), ""))
	}

	if len(m.Spec.Phases) == 0 {
		allErrs = append(allErrs,
			field.Required(spec.Child("phases"), ""))
	}
	phaseNames := map[string]struct{}{}
	specPhases := spec.Child("phases")
	for i, phase := range m.Spec.Phases {
		if _, alreadyExists := phaseNames[phase.Name]; alreadyExists {
			allErrs = append(allErrs,
				field.Invalid(specPhases.Index(i).Child("name"), phase.Name, "must be unique"))
		}
		phaseNames[phase.Name] = struct{}{}
	}

	specProbes := field.NewPath("spec").Child("availabilityProbes")
	if len(m.Spec.AvailabilityProbes) == 0 {
		allErrs = append(allErrs,
			field.Required(specProbes, ""))
	}
	for i, probe := range m.Spec.AvailabilityProbes {
		if len(probe.Probes) == 0 {
			allErrs = append(allErrs,
				field.Required(specProbes.Index(i).Child("probes"), ""))
		}
	}

	testTemplate := field.NewPath("test").Child("template")
	for i, template := range m.Test.Template {
		el := validation.IsConfigMapKey(template.Name)
		if len(el) > 0 {
			allErrs = append(allErrs,
				field.Invalid(testTemplate.Index(i).Child("name"), template.Name, allErrs.ToAggregate().Error()))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs.ToAggregate()
}

func init() {
	SchemeBuilder.Register(&PackageManifest{})
}
