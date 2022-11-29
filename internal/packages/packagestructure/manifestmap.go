package packagestructure

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagebytes"
)

// Maps filenames to kubernetes manifests.
type ManifestMap map[string][]unstructured.Unstructured

// Converts the PackageImage back into a FileMap.
func (mm ManifestMap) ToFileMap() (packagebytes.FileMap, error) {
	fm := packagebytes.FileMap{}

	for path, objects := range mm {
		var manifestBytes bytes.Buffer
		for i, object := range objects {
			objectBytes, err := yaml.Marshal(object)
			if err != nil {
				return nil, fmt.Errorf("marshal YAML for File %s: %w", path, err)
			}

			if i > 0 {
				if _, err := manifestBytes.Write([]byte("---\n")); err != nil {
					return nil, fmt.Errorf("write to buffer: %w", err)
				}
			}
			if _, err := manifestBytes.Write(objectBytes); err != nil {
				return nil, fmt.Errorf("write to buffer: %w", err)
			}
		}

		fm[path] = manifestBytes.Bytes()
	}

	return fm, nil
}

type ManifestMapLoader struct{}

func NewManifestMapLoader() *ManifestMapLoader {
	return &ManifestMapLoader{}
}

func (l *ManifestMapLoader) FromFileMap(ctx context.Context, fileMap packagebytes.FileMap) (ManifestMap, error) {
	mm := ManifestMap{}
	for path, content := range fileMap {
		objects, err := ObjectsFromBytes(ctx, path, content)
		if err != nil {
			return nil, err
		}
		if len(objects) == 0 {
			continue
		}
		mm[path] = objects
	}
	return mm, nil
}

func ObjectsFromBytes(ctx context.Context, path string, content []byte) (
	objects []unstructured.Unstructured, err error,
) {
	if !packages.IsYAMLFile(path) {
		// not a YAML file, skip
		return nil, nil
	}
	if packages.IsManifestFile(path) {
		// skip manifest file, this is handled in it's own loader.
		return nil, nil
	}

	// Trim empty starting and ending objects
	content = bytes.Trim(content, "---\n")

	// Split for every included yaml document.
	for i, yamlDocument := range bytes.Split(content, []byte("---\n")) {
		obj := unstructured.Unstructured{}
		if err := yaml.Unmarshal(yamlDocument, &obj); err != nil {
			return nil, NewInvalidError(Violation{
				Reason:  ViolationReasonInvalidYAML,
				Details: err.Error(),
				Location: &ViolationLocation{
					Path:          path,
					DocumentIndex: pointer.Int(i),
				},
			})
		}

		if len(obj.Object) == 0 {
			continue
		}
		objects = append(objects, obj)
	}
	return
}
