package packagekickstart

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"pkg.package-operator.run/cardboard/kubeutils/kubemanifests"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/internal/packagekickstart/parametrize"
	"package-operator.run/internal/packages/internal/packagekickstart/presets"
	"package-operator.run/internal/packages/internal/packagetypes"
)

type KickstartResult struct {
	ObjectCount             int
	GroupKindsWithoutProbes []schema.GroupKind
}

func Kickstart(
	_ context.Context, pkgName string,
	objects []unstructured.Unstructured,
	paramFlags []string,
) (
	*packagetypes.RawPackage, KickstartResult, error,
) {
	res := KickstartResult{}
	rawPkg := &packagetypes.RawPackage{
		Files: packagetypes.Files{},
	}

	paramOpts := presets.ParametrizeOptionsFromFlags(paramFlags)

	// Process objects.
	var (
		objCount     int
		duplicateMap = map[objectIdentity]struct{}{}
		usedPhases   = map[string]struct{}{}
		usedGKs      = map[schema.GroupKind]struct{}{}
		scheme       = &v1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]v1.JSONSchemaProps{},
		}
		imageContainer = &presets.ImageContainer{}

		namespacesFromObjects = map[string]struct{}{}
		namespaceObjectsFound = map[string]struct{}{}
	)
	for _, obj := range objects {
		gk := obj.GroupVersionKind().GroupKind()
		phase := presets.DeterminePhase(gk)

		namespacedName, err := parseObjectMeta(obj)
		if err != nil {
			return nil, res, fmt.Errorf("parsing namespace and name: %w", err)
		}

		validatedGroupKind, err := parseTypeMeta(obj)
		if err != nil {
			return nil, res, fmt.Errorf("parsing groupKind: %w", err)
		}

		// Validate that Object is not a duplicate.
		oid := objectIdentity{
			ObjectKey: namespacedName,
			GroupKind: validatedGroupKind,
		}
		if _, ok := duplicateMap[oid]; ok {
			return nil, res, &ObjectIsDuplicateError{obj}
		}
		duplicateMap[oid] = struct{}{}

		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[manifestsv1alpha1.PackagePhaseAnnotation] = phase
		obj.SetAnnotations(annotations)

		// Remember namespace assignments.
		if gk == namespaceGK {
			namespaceObjectsFound[obj.GetName()] = struct{}{}
		}
		if ns := obj.GetNamespace(); len(ns) > 0 {
			namespacesFromObjects[obj.GetNamespace()] = struct{}{}
		}
		usedPhases[phase] = struct{}{}
		usedGKs[gk] = struct{}{}
		objCount++

		// Parametrization.
		if b, ok, err := presets.Parametrize(obj, scheme, imageContainer, paramOpts); err != nil {
			return nil, res, fmt.Errorf("parametrizing: %w", err)
		} else if ok {
			addFileWithCollisionPrevention(rawPkg.Files, phase, oid, b, "yaml.gotmpl")
			continue
		}

		b, err := yaml.Marshal(obj.Object)
		if err != nil {
			return nil, res, fmt.Errorf("marshalling YAML: %w", err)
		}
		addFileWithCollisionPrevention(rawPkg.Files, phase, oid, b, "yaml")
	}

	// Add missing Namespaces
	err := addMissingNamespaces(
		paramOpts, scheme, rawPkg,
		namespacesFromObjects,
		namespaceObjectsFound, usedPhases,
	)
	if err != nil {
		return nil, res, fmt.Errorf("adding missing namespaces: %w", err)
	}

	// Generate Manifest
	var phases []manifestsv1alpha1.PackageManifestPhase
	for _, phase := range presets.OrderedPhases {
		if _, ok := usedPhases[string(phase)]; ok {
			phases = append(phases, manifestsv1alpha1.PackageManifestPhase{Name: string(phase)})
		}
	}

	probes := []corev1alpha1.ObjectSetProbe{}
	gksWithoutProbes := map[schema.GroupKind]struct{}{}
	for gk := range usedGKs {
		if presets.NoProbe(gk) {
			continue
		}
		probe, ok := presets.DetermineProbe(gk)
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

func KickstartFromBytes(ctx context.Context, pkgName string, c []byte, paramFlags []string) (
	*packagetypes.RawPackage, KickstartResult, error,
) {
	objects, err := kubemanifests.LoadKubernetesObjectsFromBytes(c)
	if err != nil {
		return nil, KickstartResult{},
			fmt.Errorf("loading Kubernetes manifests: %w", err)
	}
	return Kickstart(ctx, pkgName, objects, paramFlags)
}

type objectIdentity struct {
	client.ObjectKey
	schema.GroupKind
}

func addFileWithCollisionPrevention(
	f packagetypes.Files, phase string,
	id objectIdentity, b []byte,
	suffix string,
) {
	// Generate unique object filename in phase folder.
	// Use pattern `$phase/$name.$kind-$counter.yaml`.
	counter := 0
	var path string
	for {
		if counter == 0 {
			path = filepath.Join(phase,
				fmt.Sprintf("%s.%s.%s",
					strings.ToLower(id.Name),
					strings.ToLower(id.Kind), suffix),
			)
		} else {
			path = filepath.Join(phase,
				fmt.Sprintf("%s.%s-%d.%s",
					strings.ToLower(id.Name),
					strings.ToLower(id.Kind),
					counter, suffix),
			)
		}
		if _, ok := f[path]; !ok {
			// Found unique name.
			break
		}
		counter++
	}
	f[path] = b
}

var namespaceGK = schema.GroupKind{
	Kind: "Namespace",
}

// Add missing Namespace objects to the kickstarted package.
func addMissingNamespaces(
	opts presets.ParametrizeOptions,
	scheme *v1.JSONSchemaProps,
	rawPkg *packagetypes.RawPackage,
	namespacesFromObjects, namespaceObjectsFound, usedPhases map[string]struct{},
) error {
	// Create files for missing namespaces.
	//nolint:prealloc
	var namespaces []string
	for nsName := range namespacesFromObjects {
		_, ok := namespaceObjectsFound[nsName]
		if ok {
			continue
		}

		phase := string(presets.PhaseNamespaces)
		usedPhases[phase] = struct{}{}
		ns := unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": nsName,
					"annotations": map[string]any{
						manifestsv1alpha1.PackagePhaseAnnotation: phase,
					},
				},
			},
		}
		path := filepath.Join(phase,
			fmt.Sprintf("%s.%s.yaml", strings.ReplaceAll(
				ns.GetName(), string(filepath.Separator), "-"),
				"namespace"))
		var (
			b   []byte
			err error
		)
		if opts.Namespaces {
			path = filepath.Join(phase,
				fmt.Sprintf("%s.%s.yaml.gotmpl", strings.ReplaceAll(
					ns.GetName(), string(filepath.Separator), "-"),
					"namespace"))
			b, err = parametrize.Execute(ns, parametrize.Pipeline(
				fmt.Sprintf("default (index .config.namespaces %q) .config.namespace", nsName), "metadata.name"))
		} else {
			b, err = yaml.Marshal(ns.Object)
		}
		if err != nil {
			return fmt.Errorf("marshalling YAML: %w", err)
		}
		rawPkg.Files[path] = b
		namespaces = append(namespaces, nsName)
	}

	if opts.Namespaces {
		scheme.Properties["namespace"] = v1.JSONSchemaProps{
			Type: "string",
			Default: &v1.JSON{
				Raw: []byte(`""`),
			},
		}
		if len(namespaces) > 0 {
			scheme.Properties["namespaces"] = v1.JSONSchemaProps{
				Type:       "object",
				Properties: map[string]v1.JSONSchemaProps{},
				Default: &v1.JSON{
					Raw: []byte("{}"),
				},
			}
		}
		for _, ns := range namespaces {
			nsJSON, err := json.Marshal(ns)
			if err != nil {
				return err
			}
			scheme.Properties["namespaces"].Properties[ns] = v1.JSONSchemaProps{
				Type: "string",
				Default: &v1.JSON{
					Raw: nsJSON,
				},
			}
		}
	}
	return nil
}
