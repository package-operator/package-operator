package packagekickstart

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	ctx := context.Background()
	rawPkg, res, err := KickstartFromBytes(ctx, "my-pkg", []byte(manifest))
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

	ctx := context.Background()
	rawPkg, res, err := KickstartFromBytes(ctx, "my-pkg", []byte(manifest))
	require.NoError(t, err)
	assert.Equal(t, 2, res.ObjectCount)
	assert.Len(t, rawPkg.Files, 3)
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
			ctx := context.Background()

			wd, err := os.Getwd()
			require.NoError(t, err)
			bytes, err := os.ReadFile(filepath.Join(wd, "testdata", "errorreporting", tcase.filename))
			require.NoError(t, err)

			_, _, err = KickstartFromBytes(ctx, "my-pkg", bytes)
			if tcase.expectedErrorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tcase.expectedErrorContains)
			}
		})
	}
}
