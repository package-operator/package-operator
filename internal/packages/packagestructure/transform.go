package packagestructure

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/utils"
)

type Transformer interface {
	Transform(ctx context.Context, packageContent *PackageContent) error
}

var (
	_ Transformer = (TransformerList)(nil)
	_ Transformer = (*CommonObjectLabelsTransformer)(nil)
)

// Runs a list of Transformer over the given content.
type TransformerList []Transformer

func (l TransformerList) Transform(ctx context.Context, packageContent *PackageContent) error {
	for _, t := range l {
		if err := t.Transform(ctx, packageContent); err != nil {
			return err
		}
	}
	return nil
}

type TransformEachObjectFn func(
	ctx context.Context, path string, index int,
	packageManifest *manifestsv1alpha1.PackageManifest,
	obj *unstructured.Unstructured,
) error

func TransformEachObject(ctx context.Context, packageContent *PackageContent, transform TransformEachObjectFn) error {
	for path, objects := range packageContent.Manifests {
		for i := range objects {
			if err := transform(
				ctx, path, i, packageContent.PackageManifest,
				&packageContent.Manifests[path][i]); err != nil {
				return err
			}
		}
	}
	return nil
}

type CommonObjectLabelsTransformer struct {
	Package metav1.Object
}

func (t CommonObjectLabelsTransformer) Transform(ctx context.Context, packageContent *PackageContent) error {
	return TransformEachObject(ctx, packageContent, t.transform)
}

func (t *CommonObjectLabelsTransformer) transform(
	ctx context.Context, path string, index int,
	packageManifest *manifestsv1alpha1.PackageManifest,
	obj *unstructured.Unstructured,
) error {
	obj.SetLabels(
		utils.MergeKeysFrom(
			obj.GetLabels(),
			commonLabels(packageManifest, t.Package.GetName())))
	return nil
}

func commonLabels(manifest *manifestsv1alpha1.PackageManifest, packageName string) map[string]string {
	return map[string]string{
		manifestsv1alpha1.PackageLabel:         manifest.Name,
		manifestsv1alpha1.PackageInstanceLabel: packageName,
	}
}
