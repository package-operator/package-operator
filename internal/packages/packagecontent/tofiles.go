package packagecontent

import (
	"bytes"
	"fmt"

	"sigs.k8s.io/yaml"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
)

func FilesFromPackage(pkg *Package) (Files, error) {
	files := Files{}

	for path, objects := range pkg.Objects {
		var fileBytes bytes.Buffer
		for i, object := range objects {
			objectBytes, err := yaml.Marshal(object)
			if err != nil {
				return nil, fmt.Errorf("marshal YAML for File %s: %w", path, err)
			}

			if i > 0 {
				_, _ = fileBytes.Write([]byte("---\n"))
			}
			_, _ = fileBytes.Write(objectBytes)
		}

		files[path] = fileBytes.Bytes()
	}

	// ensure GVK is set
	pkg.PackageManifest.SetGroupVersionKind(packages.PackageManifestGroupKind.WithVersion(manifestsv1alpha1.GroupVersion.Version))
	packageManifestBytes, err := yaml.Marshal(pkg.PackageManifest)
	if err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	files[packages.PackageManifestFile] = packageManifestBytes
	return files, nil
}
