package packagecontent

import (
	"sort"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func TemplateSpecFromPackage(pkg *Package) (templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	templateSpec.AvailabilityProbes = pkg.PackageManifest.Spec.AvailabilityProbes

	objectsByPhase := map[string][]corev1alpha1.ObjectSetObject{}
	for _, objects := range pkg.Objects {
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
			objectsByPhase[phase] = append(objectsByPhase[phase], corev1alpha1.ObjectSetObject{Object: object})
		}
	}

	for _, phase := range pkg.PackageManifest.Spec.Phases {
		phase := corev1alpha1.ObjectSetTemplatePhase{Name: phase.Name, Class: phase.Class, Objects: objectsByPhase[phase.Name]}

		if len(phase.Objects) == 0 {
			// empty phases may happen due to templating for scope or topology restrictions.
			continue
		}

		// sort objects by name to ensure we are getting deterministic output.
		sort.Slice(phase.Objects, func(i, j int) bool { return phase.Objects[i].Object.GetName() < phase.Objects[j].Object.GetName() })
		templateSpec.Phases = append(templateSpec.Phases, phase)
	}
	return
}
