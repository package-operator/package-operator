package packagekickstart

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/internal/packagekickstart/parametrize"
	"package-operator.run/internal/packages/internal/packagetypes"
)

type KickstartResult struct {
	ObjectCount             int
	GroupKindsWithoutProbes []schema.GroupKind
}

type KickstartOptions struct {
	Parametrize []string
}

var namespaceGK = schema.GroupKind{
	Kind: "Namespace",
}

func Kickstart(_ context.Context, pkgName string, objects []unstructured.Unstructured, opts KickstartOptions) (
	*packagetypes.RawPackage, KickstartResult, error,
) {
	res := KickstartResult{}
	rawPkg := &packagetypes.RawPackage{
		Files: packagetypes.Files{},
	}

	// Process objects
	namespacesFromObjects := map[string]struct{}{}
	namespaceObjectsFound := map[string]struct{}{}

	usedPhases := map[string]struct{}{}
	usedGKs := map[schema.GroupKind]struct{}{}
	scheme := &v1.JSONSchemaProps{
		Type: "object",
	}
	imageContainer := &parametrize.ImageContainer{}
	var objCount int
	for _, obj := range objects {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		gk := obj.GroupVersionKind().GroupKind()
		phase := guessPresetPhase(gk)
		annotations[manifestsv1alpha1.PackagePhaseAnnotation] = phase
		obj.SetAnnotations(annotations)

		if gk == namespaceGK {
			namespaceObjectsFound[obj.GetName()] = struct{}{}
		}
		if ns := obj.GetNamespace(); len(ns) > 0 {
			namespacesFromObjects[obj.GetNamespace()] = struct{}{}
		}

		usedPhases[phase] = struct{}{}
		usedGKs[gk] = struct{}{}
		objCount++

		if b, ok, err := parametrize.Parametrize(obj, scheme, imageContainer, opts.Parametrize); err != nil {
			return nil, res, fmt.Errorf("parametrizing: %w", err)
		} else if ok {
			path := filepath.Join(phase,
				fmt.Sprintf("%s.%s.yaml.gotmpl", obj.GetName(),
					strings.ToLower(obj.GetKind())))
			rawPkg.Files[path] = b
			continue
		}

		path := filepath.Join(phase,
			fmt.Sprintf("%s.%s.yaml", strings.ReplaceAll(
				obj.GetName(), string(filepath.Separator), "-"),
				strings.ToLower(obj.GetKind())))
		b, err := yaml.Marshal(obj.Object)
		if err != nil {
			return nil, res, fmt.Errorf("marshalling YAML: %w", err)
		}
		rawPkg.Files[path] = b
	}

	// Create files for missing namespaces.
	for nsName := range namespacesFromObjects {
		_, ok := namespaceObjectsFound[nsName]
		if ok {
			continue
		}

		phase := string(presetPhaseNamespaces)
		usedPhases[phase] = struct{}{}
		ns := &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Annotations: map[string]string{
					manifestsv1alpha1.PackagePhaseAnnotation: phase,
				},
			},
		}
		path := filepath.Join(phase,
			fmt.Sprintf("%s.%s.yaml", strings.ReplaceAll(
				ns.GetName(), string(filepath.Separator), "-"),
				"namespace"))
		b, err := yaml.Marshal(ns)
		if err != nil {
			return nil, res, fmt.Errorf("marshalling YAML: %w", err)
		}
		rawPkg.Files[path] = b
	}

	// Generate Manifest
	var phases []manifestsv1alpha1.PackageManifestPhase
	for _, phase := range orderedPhases {
		if _, ok := usedPhases[string(phase)]; ok {
			phases = append(phases, manifestsv1alpha1.PackageManifestPhase{Name: string(phase)})
		}
	}

	probes := []corev1alpha1.ObjectSetProbe{}
	gksWithoutProbes := map[schema.GroupKind]struct{}{}
	for gk := range usedGKs {
		if _, needsNoProbe := noProbeGK[gk]; needsNoProbe {
			continue
		}
		probe, ok := getProbe(gk)
		if !ok {
			gksWithoutProbes[gk] = struct{}{}
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
				manifestsv1alpha1.PackageManifestScopeCluster,
				manifestsv1alpha1.PackageManifestScopeNamespaced,
			},
			AvailabilityProbes: probes,
			Images:             imageContainer.List(),
		},
	}
	if len(scheme.Properties) > 0 {
		manifest.Spec.Config = manifestsv1alpha1.PackageManifestSpecConfig{
			OpenAPIV3Schema: scheme,
		}
		manifest.Test = manifestsv1alpha1.PackageManifestTest{
			Template: []manifestsv1alpha1.PackageManifestTestCaseTemplate{
				{
					Name:    "defaults",
					Context: manifestsv1alpha1.TemplateContext{},
				},
			},
		}
	}

	b, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, res, fmt.Errorf("marshalling PackageManifest YAML: %w", err)
	}
	rawPkg.Files[packagetypes.PackageManifestFilename+".yaml"] = b

	// Result stats
	for gk := range gksWithoutProbes {
		res.GroupKindsWithoutProbes = append(res.GroupKindsWithoutProbes, gk)
	}
	res.ObjectCount = objCount

	return rawPkg, res, nil
}

func KickstartFromBytes(ctx context.Context, pkgName string, c []byte, opts KickstartOptions) (
	*packagetypes.RawPackage, KickstartResult, error,
) {
	objects, err := kubemanifests.LoadKubernetesObjectsFromBytes(c)
	if err != nil {
		return nil, KickstartResult{},
			fmt.Errorf("loading Kubernetes manifests: %w", err)
	}
	return Kickstart(ctx, pkgName, objects, opts)
}
