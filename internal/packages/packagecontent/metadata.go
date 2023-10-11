package packagecontent

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

func metadata(objectFiles map[string][]unstructured.Unstructured) (Metadata, error) {
	meta := Metadata{}

	// GKs from parsed objects.
	managedGK := sets.Set[schema.GroupKind]{}
	externalGK := sets.Set[schema.GroupKind]{}

	for _, objects := range objectFiles {
		for _, obj := range objects {
			obj := obj
			gk := obj.GroupVersionKind().GroupKind()
			if obj.GetAnnotations()[manifestsv1alpha1.PackageExternalObjectAnnotation] == "True" {
				externalGK.Insert(gk)
			} else {
				managedGK.Insert(gk)
			}
		}
	}

	meta.ManagedObjectTypes = managedGK.UnsortedList()
	meta.ExternalObjectTypes = externalGK.UnsortedList()

	return meta, nil
}
