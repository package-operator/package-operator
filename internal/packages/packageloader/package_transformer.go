package packageloader

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages/packagecontent"
)

var _ Transformer = (*PackageTransformer)(nil)

type PackageTransformer struct{ Package metav1.Object }

func (t *PackageTransformer) TransformPackage(ctx context.Context, packageContent *packagecontent.Package) error {
	return TransformEachObject(ctx, packageContent, t.transform)
}

func (t *PackageTransformer) transform(
	_ context.Context, _ string, _ int, packageManifest *manifestsv1alpha1.PackageManifest, obj *unstructured.Unstructured,
) error {
	obj.SetLabels(labels.Merge(obj.GetLabels(), commonLabels(packageManifest, t.Package.GetName())))

	return nil
}

func commonLabels(manifest *manifestsv1alpha1.PackageManifest, packageName string) map[string]string {
	return map[string]string{manifestsv1alpha1.PackageLabel: manifest.Name, manifestsv1alpha1.PackageInstanceLabel: packageName}
}

type TransformEachObjectFn func(
	ctx context.Context, path string, index int, packageManifest *manifestsv1alpha1.PackageManifest, obj *unstructured.Unstructured) error

func TransformEachObject(ctx context.Context, packageContent *packagecontent.Package, transform TransformEachObjectFn) error {
	for path, objects := range packageContent.Objects {
		for i := range objects {
			if err := transform(
				ctx, path, i, packageContent.PackageManifest,
				&packageContent.Objects[path][i]); err != nil {
				return err
			}
		}
	}
	return nil
}
