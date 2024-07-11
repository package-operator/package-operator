package kickstart

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

type Kickstarter struct {
	stdin io.Reader
}

func NewKickstarter(stdin io.Reader) *Kickstarter {
	return &Kickstarter{stdin: stdin}
}

func (k *Kickstarter) KickStart(
	ctx context.Context,
	pkgName string,
	inputs []string,
) (msg string, err error) {
	if err := os.Mkdir(pkgName, os.ModePerm); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	usedPhases := map[string]struct{}{}
	usedGKs := map[schema.GroupKind]struct{}{}
	var objCount int
	for _, input := range inputs {
		objects, err := k.getInput(input)
		if err != nil {
			return "", fmt.Errorf("get input: %w", err)
		}

		for _, obj := range objects {
			phase, gk, err := k.processObject(pkgName, obj)
			if err != nil {
				return "", fmt.Errorf("processing object: %w", err)
			}
			usedPhases[phase] = struct{}{}
			usedGKs[gk] = struct{}{}
		}
		objCount = objCount + len(objects)
	}

	// Write Manifest
	var phases []manifestsv1alpha1.PackageManifestPhase
	for _, phase := range orderedPhases {
		if _, ok := usedPhases[string(phase)]; ok {
			phases = append(phases, manifestsv1alpha1.PackageManifestPhase{Name: string(phase)})
		}
	}
	var probes []corev1alpha1.ObjectSetProbe
	for gk := range usedGKs {
		probe, ok := getProbe(gk)
		if !ok {
			continue
		}
		probes = append(probes, probe)
	}
	manifest := &manifestsv1alpha1.PackageManifest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageManifest",
			APIVersion: "manifests.package-operator.run/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: pkgName,
		},
		Spec: manifestsv1alpha1.PackageManifestSpec{
			Phases: phases,
			Scopes: []manifestsv1alpha1.PackageManifestScope{
				// TODO: new option?
				manifestsv1alpha1.PackageManifestScopeCluster,
				manifestsv1alpha1.PackageManifestScopeNamespaced,
			},
			AvailabilityProbes: probes,
		},
	}
	b, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("marshalling PackageManifest YAML: %w", err)
	}
	if err := os.WriteFile(filepath.Join(pkgName, "manifest.yaml"), b, os.ModePerm); err != nil {
		return "", fmt.Errorf("writing PackageManifest: %w", err)
	}

	return fmt.Sprintf("Kickstarted the %q package with %d objects.", pkgName, objCount), nil
}

func (k *Kickstarter) processObject(
	pkgName string, obj unstructured.Unstructured,
) (phase string, gk schema.GroupKind, err error) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	gk = obj.GroupVersionKind().GroupKind()
	phase = guessPresetPhase(gk)
	annotations[manifestsv1alpha1.PackagePhaseAnnotation] = phase
	obj.SetAnnotations(annotations)

	path := filepath.Join(pkgName, phase,
		fmt.Sprintf("%s.%s.yaml", obj.GetName(), strings.ToLower(obj.GetKind())))
	b, err := yaml.Marshal(obj.Object)
	if err != nil {
		return phase, gk, fmt.Errorf("marshalling YAML: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return phase, gk, fmt.Errorf("creating directory: %w", err)
	}
	return phase, gk, os.WriteFile(path, b, os.ModePerm)
}

func (k *Kickstarter) getInput(input string) (
	[]unstructured.Unstructured, error,
) {
	var reader io.Reader
	switch {
	case input == "-":
		// from stdin
		reader = k.stdin

	case strings.Index(input, "http://") == 0 ||
		strings.Index(input, "https://") == 0:
		// from HTTP(S)
		resp, err := http.Get(input)
		if err != nil {
			return nil, fmt.Errorf("HTTP get: %w", err)
		}
		defer resp.Body.Close()
		reader = resp.Body

	default:
		// Files or Folders
		matches, err := expandIfFilePattern(input)
		if err != nil {
			return nil, fmt.Errorf("expand pattern: %w", err)
		}

		var objects []unstructured.Unstructured
		for _, match := range matches {
			i, err := os.Stat(match)
			if err != nil {
				return nil, fmt.Errorf("accessing: %w", err)
			}
			var (
				matchObjs []unstructured.Unstructured
			)
			if i.IsDir() {
				matchObjs, err = kubemanifests.LoadKubernetesObjectsFromFolder(match)
			} else {
				matchObjs, err = kubemanifests.LoadKubernetesObjectsFromFile(match)
			}
			if err != nil {
				return nil, fmt.Errorf("loading kubernetes objects: %w", err)
			}
			objects = append(objects, matchObjs...)
		}
		return objects, nil
	}

	if reader == nil {
		return nil, nil
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}
	return kubemanifests.LoadKubernetesObjectsFromBytes(content)
}

// expandIfFilePattern returns all the filenames that match the input pattern
// or the filename if it is a specific filename and not a pattern.
// If the input is a pattern and it yields no result it will result in an error.
func expandIfFilePattern(pattern string) ([]string, error) {
	if _, err := os.Stat(pattern); os.IsNotExist(err) {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) == 0 {
			return nil, fmt.Errorf(os.ErrNotExist.Error(), pattern)
		}
		if err == filepath.ErrBadPattern {
			return nil, fmt.Errorf("pattern %q is not valid: %v", pattern, err)
		}
		return matches, err
	}
	return []string{pattern}, nil
}

// Determines a phase using the objects Group Kind from a list or presets.
func guessPresetPhase(gk schema.GroupKind) string {
	phase, ok := gkPhaseMap[gk]
	if !ok {
		return string(presetPhaseOther)
	}
	return string(phase)
}

type presetPhase string

const (
	presetPhaseNamespaces presetPhase = "namespaces"
	presetPhasePolicies   presetPhase = "policies"
	presetPhaseRBAC       presetPhase = "rbac"
	presetPhaseCRDs       presetPhase = "crds"
	presetPhaseStorage    presetPhase = "storage"
	presetPhaseDeploy     presetPhase = "deploy"
	presetPhasePublish    presetPhase = "publish"
	// anything else that is not explicitly sorted into a phase.
	presetPhaseOther presetPhase = "other"
)

var orderedPhases = []presetPhase{
	presetPhaseNamespaces,
	presetPhasePolicies,
	presetPhaseRBAC,
	presetPhaseCRDs,
	presetPhaseStorage,
	presetPhaseDeploy,
	presetPhasePublish,
	presetPhaseOther,
}

var gkPhaseMap = map[schema.GroupKind]presetPhase{}
var phaseGKMap = map[presetPhase][]schema.GroupKind{
	presetPhaseNamespaces: {
		{Kind: "Namespace"},
	},

	presetPhasePolicies: {
		{Kind: "ResourceQuota"},
		{Kind: "LimitRange"},
		{Kind: "PriorityClass", Group: "scheduling.k8s.io"},
		{Kind: "NetworkPolicy", Group: "networking.k8s.io"},
		{Kind: "HorizontalPodAutoscaler", Group: "autoscaling"},
		{Kind: "PodDisruptionBudget", Group: "policy"},
	},

	presetPhaseRBAC: {
		{Kind: "ServiceAccount", Group: ""},
		{Kind: "Role", Group: "rbac.authorization.k8s.io"},
		{Kind: "RoleRolebinding", Group: "rbac.authorization.k8s.io"},
		{Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"},
		{Kind: "ClusterRoleRolebinding", Group: "rbac.authorization.k8s.io"},
	},

	presetPhaseCRDs: {
		{Kind: "CustomResourceDefinition", Group: "apiextensions.k8s.io"},
	},

	presetPhaseStorage: {
		{Kind: "PersistentVolume"},
		{Kind: "PersistentVolumeClaim"},
		{Kind: "StorageClass", Group: "storage.k8s.io"},
	},

	presetPhaseDeploy: {
		{Kind: "Deployment", Group: "apps"},
		{Kind: "DaemonSet", Group: "apps"},
		{Kind: "StatefulSet", Group: "apps"},
		{Kind: "ReplicaSet"},
		{Kind: "Pod"}, // probing complicated, may be either Completed or Available.
		{Kind: "Job", Group: "batch"},
		{Kind: "CronJob", Group: "batch"},
		{Kind: "Service"},
		{Kind: "Secret"},
		{Kind: "ConfigMap"},
	},

	presetPhasePublish: {
		{Kind: "Ingress", Group: "networking.k8s.io"},
		{Kind: "APIService", Group: "apiregistration.k8s.io"},
		{Kind: "Route", Group: "route.openshift.io"},
		{Kind: "MutatingWebhookConfiguration", Group: "admissionregistration.k8s.io"},
		{Kind: "ValidatingWebhookConfiguration", Group: "admissionregistration.k8s.io"},
	},
}

// Determines probes required for the given Group Kind.
func getProbe(gk schema.GroupKind) (corev1alpha1.ObjectSetProbe, bool) {
	probes, ok := gkProbes[gk]
	if !ok {
		return corev1alpha1.ObjectSetProbe{}, false
	}
	return corev1alpha1.ObjectSetProbe{
		Selector: corev1alpha1.ProbeSelector{
			Kind: &corev1alpha1.PackageProbeKindSpec{
				Group: gk.Group,
				Kind:  gk.Kind,
			},
		},
		Probes: probes,
	}, true
}

var gkProbes = map[schema.GroupKind][]corev1alpha1.Probe{
	{
		Kind: "Deployment", Group: "apps",
	}: {
		availableProbe,
		replicasUpdatedProbe,
	},
	{
		Kind: "StatefulSet", Group: "apps",
	}: {
		availableProbe,
		replicasUpdatedProbe,
	},
	{
		Kind: "DaemonSet", Group: "apps",
	}: {
		{
			FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
				FieldA: ".status.desiredNumberScheduled",
				FieldB: ".status.numberAvailable",
			},
		},
	},
	{
		Kind: "ReplicaSet", Group: "apps",
	}: {
		availableProbe,
		replicasUpdatedProbe,
	},
	{
		Kind:  "CustomResourceDefinition",
		Group: "apiextensions.k8s.io",
	}: {
		{
			Condition: &corev1alpha1.ProbeConditionSpec{
				Type:   "Established",
				Status: string(metav1.ConditionTrue),
			},
		},
	},
	{
		Kind:  "Job",
		Group: "batch",
	}: {
		{
			Condition: &corev1alpha1.ProbeConditionSpec{
				Type:   "Complete",
				Status: string(metav1.ConditionTrue),
			},
		},
	},
	{
		Kind:  "Route",
		Group: "route.openshift.io",
	}: {
		{
			CEL: &corev1alpha1.ProbeCELSpec{
				Message: "not all ingress points are reporting ready",
				Rule:    `self.status.ingress.all(i, i.conditions.all(c, c.type == "Ready" && c.status == "True"))`,
			},
		},
	},
	{
		Kind:  "PersistentVolumeClaim",
		Group: "",
	}: {
		{
			CEL: &corev1alpha1.ProbeCELSpec{
				Message: "is not yet Bound",
				Rule:    `self.status.phase == "Bound"`,
			},
		},
	},
	{
		Kind:  "ClusterServiceVersion",
		Group: "operators.coreos.com",
	}: {
		{
			CEL: &corev1alpha1.ProbeCELSpec{
				Message: "CSV not succeeded",
				Rule:    `self.status.phase == "Succeeded"`,
			},
		},
	},
}

// Checks if the Available Condition is True.
var availableProbe = corev1alpha1.Probe{
	Condition: &corev1alpha1.ProbeConditionSpec{
		Type:   "Available",
		Status: string(metav1.ConditionTrue),
	},
}

// Checks if all replicas have been updated.
// Works for StatefulSets, Deployments and ReplicaSets.
var replicasUpdatedProbe = corev1alpha1.Probe{
	FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
		FieldA: ".status.updatedReplicas",
		FieldB: ".status.replicas",
	},
}

func init() {
	for phase, gks := range phaseGKMap {
		for _, gk := range gks {
			gkPhaseMap[gk] = phase
		}
	}
}
