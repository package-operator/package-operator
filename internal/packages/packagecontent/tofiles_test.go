package packagecontent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packagecontent"
)

func TestFilesFromPackage(t *testing.T) {
	t.Parallel()

	pkg := &packagecontent.Package{
		PackageManifest: &manifestsv1alpha1.PackageManifest{ObjectMeta: metav1.ObjectMeta{}},
		Objects:         map[string][]unstructured.Unstructured{"test.yaml": {{}, {}}},
	}

	expectedPackageManifest := `apiVersion: manifests.package-operator.run/v1alpha1
kind: PackageManifest
metadata:
  creationTimestamp: null
spec:
  config: {}
  images: null
  phases: null
  scopes: null
test: {}
`

	expectedTestYaml := `Object: null
---
Object: null
`

	files, err := packagecontent.FilesFromPackage(pkg)
	require.NoError(t, err)

	require.Len(t, files, 2)
	assert.Equal(t, expectedTestYaml, string(files["test.yaml"]))
	assert.Equal(t, expectedPackageManifest, string(files[packages.PackageManifestFile]))
}
