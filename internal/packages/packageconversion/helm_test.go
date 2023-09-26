package packageconversion

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/packages/packageimport"
)

func TestHelm(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pkg",
				Namespace: "test123",
			},
			Spec: corev1alpha1.PackageSpec{
				Config: &runtime.RawExtension{
					Raw: []byte(`{"image":{"tag":"v123"}}`),
				},
			},
		},
	}
	helmFiles, err := packageimport.Folder(ctx, "./testdata")
	require.NoError(t, err)

	content, err := Helm(ctx, pkg, helmFiles)
	require.NoError(t, err)
	assert.NotNil(t, content)

	if assert.NotNil(t, content.PackageManifest) {
		m := content.PackageManifest
		// helm chart name
		assert.Equal(t, "test", m.Name)
	}

	if assert.Len(t, content.Objects[manifestsFile], 1) {
		obj := content.Objects[manifestsFile][0]
		assert.Equal(t, "Deployment", obj.GroupVersionKind().Kind)
		containers, ok, err := unstructured.NestedSlice(
			obj.Object, "spec", "template", "spec", "containers")
		require.NoError(t, err)
		assert.True(t, ok)

		c := containers[0].(map[string]interface{})
		image, _, _ := unstructured.NestedString(c, "image")
		assert.Equal(t, "nginx:v123", image)
	}
}
