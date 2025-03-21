package packagevalidation

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"package-operator.run/internal/apis/manifests"
)

func TestObjectPhaseAnnotationValidator(t *testing.T) {
	t.Parallel()

	opav := &ObjectPhaseAnnotationValidator{}

	failObj := unstructured.Unstructured{}
	failObj.SetAnnotations(map[string]string{
		manifests.PackagePhaseAnnotation: "something",
	})

	okObj := unstructured.Unstructured{}
	okObj.SetAnnotations(map[string]string{
		manifests.PackagePhaseAnnotation: "deploy",
	})

	ctx := t.Context()
	manifest := &manifests.PackageManifest{
		Spec: manifests.PackageManifestSpec{
			Phases: []manifests.PackageManifestPhase{{Name: "deploy"}},
		},
	}
	err := opav.ValidateObjects(
		ctx, manifest,
		map[string][]unstructured.Unstructured{
			"test.yaml": {{}, failObj, okObj},
		})

	require.EqualError(t, err, `Missing package-operator.run/phase Annotation in test.yaml idx 0
Phase name not found in manifest in test.yaml idx 1`)
}

func TestObjectDuplicateValidator(t *testing.T) {
	t.Parallel()

	odv := &ObjectDuplicateValidator{}

	obj := unstructured.Unstructured{}
	obj.SetAnnotations(map[string]string{
		manifests.PackagePhaseAnnotation: "something",
	})

	ctx := t.Context()
	manifest := &manifests.PackageManifest{}
	err := odv.ValidateObjects(
		ctx, manifest,
		map[string][]unstructured.Unstructured{
			"test.yaml": {{}, obj},
		})
	require.EqualError(t, err, "Duplicate Object in test.yaml idx 1")
}

func TestObjectGVKValidator(t *testing.T) {
	t.Parallel()

	ogvkv := &ObjectGVKValidator{}

	okObj := unstructured.Unstructured{}
	okObj.SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	})

	ctx := t.Context()
	manifest := &manifests.PackageManifest{}
	err := ogvkv.ValidateObjects(
		ctx, manifest,
		map[string][]unstructured.Unstructured{
			"test.yaml": {{}, okObj},
		})
	require.EqualError(t, err, "GroupVersionKind not set in test.yaml idx 0")
}

func TestObjectLabelsValidator(t *testing.T) {
	t.Parallel()

	olv := &ObjectLabelsValidator{}

	failObj := unstructured.Unstructured{}
	failObj.SetLabels(map[string]string{"/123": "test"})

	ctx := t.Context()
	manifest := &manifests.PackageManifest{}
	err := olv.ValidateObjects(
		ctx, manifest,
		map[string][]unstructured.Unstructured{
			"test.yaml": {{}, failObj},
		})
	errString := `Labels invalid in test.yaml idx 1: metadata.labels: Invalid value: "/123": prefix part must be non-empty`
	require.EqualError(t, err, errString)
}
