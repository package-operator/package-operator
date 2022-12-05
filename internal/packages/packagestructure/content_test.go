package packagestructure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/packages"
)

func TestPackageContent_ToFileMap(t *testing.T) {
	packageContent := &PackageContent{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			ObjectMeta: metav1.ObjectMeta{},
		},
		Manifests: ManifestMap{
			"test.yaml": []unstructured.Unstructured{
				{}, {},
			},
		},
	}

	expectedPackageManifest := `apiVersion: manifests.package-operator.run/v1alpha1
kind: PackageManifest
metadata:
  creationTimestamp: null
spec:
  availabilityProbes: null
  phases: null
  scopes: null
test: {}
`

	expectedTestYaml := `Object: null
---
Object: null
`

	fm, err := packageContent.ToFileMap()
	require.NoError(t, err)
	if assert.Len(t, fm, 2) {
		assert.Equal(t, expectedPackageManifest, string(fm[packages.PackageManifestFile]))
		assert.Equal(t, expectedTestYaml, string(fm["test.yaml"]))
	}
}
