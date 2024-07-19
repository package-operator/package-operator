package packagekickstart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var manifest = `apiVersion: apps/v1
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

func TestKickstartFromBytes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rawPkg, res, err := KickstartFromBytes(ctx, "my-pkg", []byte(manifest), KickstartOptions{})
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
