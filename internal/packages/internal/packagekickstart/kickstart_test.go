package packagekickstart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"package-operator.run/internal/packages/internal/packagekickstart/presets"
	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestKickstartFromBytes(t *testing.T) {
	t.Parallel()

	const manifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-app
  labels:
    app: my-app
spec:
  strategy:
    type: Recreate
  replicas: 1
  selector:
    matchLabels:
      app: my-app
    spec:
      containers:
        - name: my-container
          image: quay.io/example
---
apiVersion: v1
kind: Namespace
metadata:
  name: my-app
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/audit-version: latest
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: latest
---
apiVersion: fruits/v1
kind: Banana
metadata:
  name: cavendish
spec:
  sweet: True
`

	ctx := t.Context()
	rawPkg, res, err := KickstartFromBytes(ctx, "my-pkg", []byte(manifest), nil)
	require.NoError(t, err)
	assert.Equal(t, 3, res.ObjectCount)
	if assert.Len(t, res.GroupKindsWithoutProbes, 1) {
		assert.Equal(t, schema.GroupKind{
			Group: "fruits",
			Kind:  "Banana",
		}, res.GroupKindsWithoutProbes[0])
	}
	assert.Len(t, rawPkg.Files, 4)
}

func TestKickstartFromBytes_SameNameDifferentNamespaces(t *testing.T) {
	t.Parallel()

	const manifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: aaah
  namespace: a
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: aaah
  namespace: b`

	ctx := t.Context()
	rawPkg, res, err := KickstartFromBytes(ctx, "my-pkg", []byte(manifest), []string{"namespaces"})
	require.NoError(t, err)
	assert.Equal(t, 2, res.ObjectCount)
	// both config maps + package manifest + 2 namespace objects.
	assert.Len(t, rawPkg.Files, 5)
	assert.NotEmpty(t, rawPkg.Files["deploy/aaah.configmap.yaml.gotmpl"])
	assert.NotEmpty(t, rawPkg.Files["deploy/aaah.configmap-1.yaml.gotmpl"])
	assert.NotEmpty(t, rawPkg.Files["manifest.yaml"])
	assert.NotEmpty(t, rawPkg.Files["namespaces/a.namespace.yaml.gotmpl"])
	assert.NotEmpty(t, rawPkg.Files["namespaces/b.namespace.yaml.gotmpl"])
}

type errorReportingTestCase struct {
	filename              string
	expectedErrorContains string
}

var errorReportingTestCases = []errorReportingTestCase{
	{
		filename:              "duplicate-object.yaml",
		expectedErrorContains: "duplicate object",
	},
	{
		filename:              "metadata-is-string.yaml",
		expectedErrorContains: "parsing namespace and name: object is missing metadata",
	},
	{
		filename:              "missing-apiversion.yaml",
		expectedErrorContains: "parsing groupKind: object has invalid apiVersion",
	},
	{
		filename: "missing-kind.yaml",
		// This is a bit ugly because this error comes from a kubernetes package outside of this project.
		// It should still be included to serve as a regression test
		// in case we have to start checking for a missing kind in `meta.go`.
		expectedErrorContains: "Object 'Kind' is missing",
	},
	{
		filename:              "missing-metadata.yaml",
		expectedErrorContains: "object is missing metadata",
	},
	{
		filename:              "missing-name.yaml",
		expectedErrorContains: "object is missing name",
	},
	{
		filename: "missing-namespace.yaml",
		// should not error
		expectedErrorContains: "",
	},
	{
		filename: "same-name-different-namespace.yaml",
		// should not error
		expectedErrorContains: "",
	},
}

func TestKickStartFromBytes_ErrorReporting(t *testing.T) {
	t.Parallel()
	for _, tcase := range errorReportingTestCases {
		t.Run(tcase.filename, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()

			wd, err := os.Getwd()
			require.NoError(t, err)
			bytes, err := os.ReadFile(filepath.Join(wd, "testdata", "errorreporting", tcase.filename))
			require.NoError(t, err)

			_, _, err = KickstartFromBytes(ctx, "my-pkg", bytes, nil)
			if tcase.expectedErrorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tcase.expectedErrorContains)
			}
		})
	}
}

func Test_addMissingNamespaces(t *testing.T) {
	t.Parallel()
	config := &v1.JSONSchemaProps{
		Properties: map[string]v1.JSONSchemaProps{},
	}
	rawPkg := &packagetypes.RawPackage{Files: packagetypes.Files{}}
	namespacesFromObjects := map[string]struct{}{
		"banana": {},
	}
	namespaceObjectsFound := map[string]struct{}{}
	usedPhases := map[string]struct{}{}
	err := addMissingNamespaces(presets.ParametrizeOptions{
		Namespaces: true,
	}, config, rawPkg, namespacesFromObjects,
		namespaceObjectsFound, usedPhases,
	)
	require.NoError(t, err)

	assert.Len(t, rawPkg.Files, 1)
	assert.Equal(t, `apiVersion: v1
kind: Namespace
metadata:
  annotations:
    package-operator.run/phase: namespaces
  name: {{ default (index .config.namespaces "banana") .config.namespace }}
`, string(rawPkg.Files["namespaces/banana.namespace.yaml.gotmpl"]))
}
