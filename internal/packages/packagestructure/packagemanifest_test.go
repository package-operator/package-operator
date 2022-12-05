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
			err:  "Package validation errors:\n- PackageManifest not found:\n  searched at manifest.yaml,manifest.yml",
		},
		{
			name: "invalid YAML",
			fileMap: packagebytes.FileMap{
				packages.PackageManifestFile: []byte("{xxx..,akd:::"),
			},
			err: "Package validation errors:\n- Invalid YAML in manifest.yaml:\n  error converting YAML to JSON: yaml: line 1: did not find expected node content",
		},
		{
			name: "invalid GVK",
			fileMap: packagebytes.FileMap{
				packages.PackageManifestFile: []byte("apiVersion: fruits/v1\nkind: Banana"),
			},
			err: "Package validation errors:\n- PackageManifest unknown GVK in manifest.yaml:\n  GroupKind must be PackageManifest.manifests.package-operator.run, is: Banana.fruits",
		},
		{
			name: "unsupported Version",
			fileMap: packagebytes.FileMap{
				packages.PackageManifestFile: []byte("apiVersion: manifests.package-operator.run/v23\nkind: PackageManifest"),
			},
			err: "Package validation errors:\n- PackageManifest unknown GVK in manifest.yaml:\n  unknown version v23, supported versions: v1alpha1",
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
