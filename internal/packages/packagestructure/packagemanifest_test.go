package packagestructure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/internal/packages"
	"package-operator.run/package-operator/internal/packages/packagebytes"
)

func TestPackageManifestLoader_Errors(t *testing.T) {
	l := NewPackageManifestLoader(testScheme)

	tests := []struct {
		name    string
		fileMap packagebytes.FileMap
		err     string
	}{
		{
			name: "not found",
			err:  "Package validation errors:\n- PackageManifest not found searched at manifest.yaml,manifest.yml\n",
		},
		{
			name: "invalid YAML",
			fileMap: packagebytes.FileMap{
				packages.PackageManifestFile: []byte("{xxx..,akd:::"),
			},
			err: "Package validation errors:\n- Invalid YAML error converting YAML to JSON: yaml: line 1: did not find expected node content in manifest.yaml\n",
		},
		{
			name: "invalid GVK",
			fileMap: packagebytes.FileMap{
				packages.PackageManifestFile: []byte("apiVersion: fruits/v1\nkind: Banana"),
			},
			err: "Package validation errors:\n- PackageManifest unknown GVK GroupKind must be PackageManifest.manifests.package-operator.run, is: Banana.fruits in manifest.yaml\n",
		},
		{
			name: "unsupported Version",
			fileMap: packagebytes.FileMap{
				packages.PackageManifestFile: []byte("apiVersion: manifests.package-operator.run/v23\nkind: PackageManifest"),
			},
			err: "Package validation errors:\n- PackageManifest unknown GVK unknown version v23, supported versions: v1alpha1 in manifest.yaml\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := l.FromFileMap(ctx, test.fileMap)
			require.EqualError(t, err, test.err)
		})
	}
}
