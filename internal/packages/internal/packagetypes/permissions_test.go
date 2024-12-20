package packagetypes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestPermissions(t *testing.T) {
	t.Parallel()

	files := Files{
		"xxx.yaml.gotmpl": []byte(`
apiVersion: my-group/v1alpha1
kind: MyThing
---
apiVersion: my-group/v1alpha1
kind: MyOtherThing
metadata:
  annotations:
    package-operator.run/external: "True"
---
apiVersion: v1
kind: Secret
---
apiVersion: v1
kind: ConfigMap
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    package-operator.run/external: "True"
`),
	}

	ctx := context.Background()
	perms, err := Permissions(ctx, files)
	require.NoError(t, err)
	if assert.Len(t, perms.Managed, 3) {
		assert.Contains(t, perms.Managed, schema.GroupKind{Kind: "ConfigMap"})
		assert.Contains(t, perms.Managed, schema.GroupKind{Kind: "Secret"})
		assert.Contains(t, perms.Managed, schema.GroupKind{Group: "my-group", Kind: "MyThing"})
	}
	if assert.Len(t, perms.External, 2) {
		assert.Contains(t, perms.External, schema.GroupKind{Kind: "Service"})
		assert.Contains(t, perms.External, schema.GroupKind{Group: "my-group", Kind: "MyOtherThing"})
	}
}
