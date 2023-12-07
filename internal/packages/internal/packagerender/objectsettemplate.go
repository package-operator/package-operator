package packagerender

import (
	"sort"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Renders a ObjectSetTemplateSpec from a PackageInstance to use with ObjectSet and ObjectDeployment APIs.
func RenderObjectSetTemplateSpec(pkgInstance *packagetypes.PackageInstance) (templateSpec corev1alpha1.ObjectSetTemplateSpec) {
	collector := newPhaseCollector(pkgInstance.Manifest.Spec.Phases...)
	collector.AddObjects(pkgInstance.Objects...)

	templateSpec.AvailabilityProbes = pkgInstance.Manifest.Spec.AvailabilityProbes
	templateSpec.Phases = append(templateSpec.Phases, collector.Collect()...)
	return
}

func newPhaseCollector(phases ...manifests.PackageManifestPhase) phaseCollector {
	collector := make(phaseCollector)

	for idx, phase := range phases {
		collector[phase.Name] = phaseCollectorEntry{
			Index: idx,
			Phase: corev1alpha1.ObjectSetTemplatePhase{
				Name:  phase.Name,
				Class: phase.Class,
			},
		}
	}

	return collector
}

type phaseCollector map[string]phaseCollectorEntry

type phaseCollectorEntry struct {
	Index int
	Phase corev1alpha1.ObjectSetTemplatePhase
}

func (c phaseCollector) AddObjects(objs ...unstructured.Unstructured) {
	for i, object := range objs {
		annotations := object.GetAnnotations()
		phaseAnnotation := annotations[manifestsv1alpha1.PackagePhaseAnnotation]
		isExternalObject := annotations[manifestsv1alpha1.PackageExternalObjectAnnotation] == "True"
		delete(annotations, manifestsv1alpha1.PackagePhaseAnnotation)
		delete(annotations, manifestsv1alpha1.PackageConditionMapAnnotation)
		delete(annotations, manifestsv1alpha1.PackageExternalObjectAnnotation)
		if len(annotations) == 0 {
			// This is important!
			// When submitted to the API server empty maps will be dropped.
			// Semantic equality checking is considering a nil map not equal to an empty map.
			// And if semantic equality checking fails, hash collision checks will always find a hash collision if the ObjectSlice already exists.
			annotations = nil
		}

		// Any error should have been detected by the validation stage.
		conditionMapping, err := parseConditionMapAnnotation(&objs[i])
		if err != nil {
			panic(err)
		}

		object.SetAnnotations(annotations)

		objSetObj := corev1alpha1.ObjectSetObject{
			Object:            object,
			ConditionMappings: conditionMapping,
		}

		if isExternalObject {
			c.addExternalObjects(phaseAnnotation, objSetObj)
		} else {
			c.addObjects(phaseAnnotation, objSetObj)
		}
	}
}

func (c phaseCollector) addObjects(phaseName string, objs ...corev1alpha1.ObjectSetObject) {
	entry, ok := c[phaseName]
	if !ok {
		return
	}

	entry.Phase.Objects = append(entry.Phase.Objects, objs...)

	c[phaseName] = entry
}

func (c phaseCollector) addExternalObjects(phaseName string, objs ...corev1alpha1.ObjectSetObject) {
	entry, ok := c[phaseName]
	if !ok {
		return
	}

	entry.Phase.ExternalObjects = append(entry.Phase.ExternalObjects, objs...)

	c[phaseName] = entry
}

func (c phaseCollector) Collect() []corev1alpha1.ObjectSetTemplatePhase {
	entries := make([]phaseCollectorEntry, 0, len(c))
	for _, entry := range c {
		if len(entry.Phase.Objects) == 0 && len(entry.Phase.ExternalObjects) == 0 {
			// empty phases may happen due to templating for scope or topology restrictions.
			continue
		}

		entries = append(entries, entry)
	}

	// Ensure ordering remains consistent with manifest
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Index < entries[j].Index
	})

	phases := make([]corev1alpha1.ObjectSetTemplatePhase, len(entries))

	for i, e := range entries {
		phases[i] = e.Phase
	}

	return phases
}
