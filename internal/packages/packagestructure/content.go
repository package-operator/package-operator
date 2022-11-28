package packagestructure

import (
	"fmt"
	"sort"

	"sigs.k8s.io/yaml"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagebytes"
)

// PackageContent represents the parsed content of an PKO package.
type PackageContent struct {
	PackageManifest *manifestsv1alpha1.PackageManifest
	Manifests       ManifestMap
}

// Converts the PackageContent back into a FileMap.
func (pc *PackageContent) ToFileMap() (packagebytes.FileMap, error) {
	fm, err := pc.Manifests.ToFileMap()
	if err != nil {
		return nil, err
	}

	// ensure GVK is set
	pc.PackageManifest.SetGroupVersionKind(
		packages.PackageManifestGroupKind.WithVersion(
			manifestsv1alpha1.GroupVersion.Version))
	packageManifestBytes, err := yaml.Marshal(pc.PackageManifest)
	if err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	fm[packages.PackageManifestFile] = packageManifestBytes
	return fm, nil
}

func (pc *PackageContent) ToTemplateSpec() (templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	templateSpec.AvailabilityProbes = pc.PackageManifest.Spec.AvailabilityProbes

	objectsByPhase := map[string][]corev1alpha1.ObjectSetObject{}
	for _, objects := range pc.Manifests {
		for _, object := range objects {
			annotations := object.GetAnnotations()
			phase := annotations[manifestsv1alpha1.PackagePhaseAnnotation]
			delete(annotations, manifestsv1alpha1.PackagePhaseAnnotation)
			if len(annotations) == 0 {
				// This is important!
				// When submitted to the API server empty maps will be dropped.
				// Semantic equality checking is considering a nil map to ne not equal to an empty map.
				// And if semantic equality checking fails, hash collision checks will always find a hash collision if the ObjectSlice already exists.
				annotations = nil
			}
			object.SetAnnotations(annotations)
			objectsByPhase[phase] = append(objectsByPhase[phase], corev1alpha1.ObjectSetObject{
				Object: object,
			})
		}
	}

	for _, phase := range pc.PackageManifest.Spec.Phases {
		phase := corev1alpha1.ObjectSetTemplatePhase{
			Name:    phase.Name,
			Class:   phase.Class,
			Objects: objectsByPhase[phase.Name],
		}

		if len(phase.Objects) == 0 {
			// empty phases may happen due to templating for scope or topology restrictions.
			continue
		}

		// sort objects by name to ensure we are getting deterministic output.
		sort.Sort(objectSetObjectByNameAscending(phase.Objects))
		templateSpec.Phases = append(templateSpec.Phases, phase)
	}
	return
}

type objectSetObjectByNameAscending []corev1alpha1.ObjectSetObject

func (a objectSetObjectByNameAscending) Len() int      { return len(a) }
func (a objectSetObjectByNameAscending) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a objectSetObjectByNameAscending) Less(i, j int) bool {
	return a[i].Object.GetName() < a[j].Object.GetName()
}
