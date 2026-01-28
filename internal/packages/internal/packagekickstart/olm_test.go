package packagekickstart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/testutil"
)

var (
	olmBundleAnnotations = `annotations:
  # Core bundle annotations.
  operators.operatorframework.io.bundle.mediatype.v1: registry+v1
  operators.operatorframework.io.bundle.manifests.v1: manifests/
  operators.operatorframework.io.bundle.metadata.v1: metadata/
  operators.operatorframework.io.bundle.package.v1: example-operator
  operators.operatorframework.io.bundle.channels.v1: alpha
  operators.operatorframework.io.metrics.builder: operator-sdk-v1.11.0+git
  operators.operatorframework.io.metrics.mediatype.v1: metrics+v1
  operators.operatorframework.io.metrics.project_layout: go.kubebuilder.io/v3

  # Annotations for testing.
  operators.operatorframework.io.test.mediatype.v1: scorecard+v1
  operators.operatorframework.io.test.config.v1: tests/scorecard/
`
	olmBundleCSV = `apiVersion: operations.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: example-operator.v0.1.0
  namespace: placeholder
spec:
  installModes:
  - supported: false
    type: OwnNamespace
  - supported: false
    type: SingleNamespace
  - supported: false
    type: MultiNamespace
  - supported: true
    type: AllNamespaces
  install:
    spec:
      deployments:
      - name: example-operator-controller-manager
        spec:
          replicas: 1
          template:
            spec:
              containers:
              - image: gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0
                name: kube-rbac-proxy
`
)

func TestImportOLMBundleImage(t *testing.T) {
	t.Parallel()

	image := testutil.BuildImage(t, map[string][]byte{
		olmMetadataFolder + "/annotations.yaml":                []byte(olmBundleAnnotations),
		olmManifestFolder + "/test.clusterserviceversion.yaml": []byte(olmBundleCSV),
	}, nil)

	ctx := context.Background()
	objects, reg, err := ImportOLMBundleImage(ctx, image)
	require.NoError(t, err)

	assert.Equal(t, "example-operator", reg.PackageName)
	assert.Equal(t, []unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"annotations": map[string]any{
						"olm.targetNamespaces": "",
					},
					"name":      "example-operator-controller-manager",
					"namespace": "example-operator-system",
				},
				"spec": map[string]any{
					"replicas": int64(1),
					"selector": nil,
					"strategy": map[string]any{},
					"template": map[string]any{
						"metadata": map[string]any{},
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":      "kube-rbac-proxy",
									"image":     "gcr.io/kubebuilder/kube-rbac-proxy:v0.8.0",
									"resources": map[string]any{},
								},
							},
						},
					},
				},
				"status": map[string]any{},
			},
		},
	}, objects)
}

func TestIsOLMBundleImage(t *testing.T) {
	t.Parallel()
	t.Run("empty image", func(t *testing.T) {
		t.Parallel()
		image := testutil.BuildImage(t, map[string][]byte{}, nil)

		isOLM, err := IsOLMBundleImage(image)
		require.NoError(t, err)

		assert.False(t, isOLM)
	})

	t.Run("package image", func(t *testing.T) {
		t.Parallel()
		image := testutil.BuildImage(t, map[string][]byte{
			"package/manifest.yaml": {11, 12},
		}, nil)

		isOLM, err := IsOLMBundleImage(image)
		require.NoError(t, err)

		assert.False(t, isOLM)
	})

	t.Run("bundle image", func(t *testing.T) {
		t.Parallel()
		image := testutil.BuildImage(t, map[string][]byte{
			olmMetadataFolder + "/annotations.yaml":                {11, 12},
			olmManifestFolder + "/test.clusterserviceversion.yaml": {11, 12},
		}, nil)

		isOLM, err := IsOLMBundleImage(image)
		require.NoError(t, err)

		assert.True(t, isOLM)
	})
}
