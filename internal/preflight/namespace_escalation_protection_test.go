package preflight

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil/restmappermock"
)

func TestNamespaceEscalation(t *testing.T) {
	clusterScoped := &unstructured.Unstructured{}

	nsOwner := &unstructured.Unstructured{}
	nsOwner.SetName("test")
	nsOwner.SetNamespace("other-ns")

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("test-ns")
	obj.SetKind("Hans")

	ctx := context.Background()

	tests := []struct {
		name               string
		ctx                context.Context
		owner, obj         client.Object
		expectedViolations []Violation
	}{
		{
			name:  "owner cluster-scoped",
			ctx:   ctx,
			owner: clusterScoped,
			obj:   obj,
		},
		{
			name: "phase set",
			ctx: NewContextWithPhase(ctx, corev1alpha1.ObjectSetTemplatePhase{
				Class: "123",
			}),
			owner: obj,
			obj:   obj,
		},
		{
			name:  "different namespace",
			ctx:   ctx,
			owner: nsOwner,
			obj:   obj,
			expectedViolations: []Violation{
				{
					Position: "Hans test-ns/test",
					Error:    "Must stay within the same namespace.",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rm := &restmappermock.RestMapperMock{}
			ne := NewNamespaceEscalation(rm)

			v, err := ne.Check(test.ctx, test.owner, test.obj)
			require.NoError(t, err)
			assert.Equal(t, test.expectedViolations, v)
		})
	}
}

func TestNamespaceEscalation_restMapper(t *testing.T) {
	rm := &restmappermock.RestMapperMock{}
	ne := NewNamespaceEscalation(rm)

	owner := &unstructured.Unstructured{}
	owner.SetNamespace("test-ns")

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Hans",
		Group:   "core",
		Version: "v4",
	})

	rm.
		On("RESTMapping").
		Return(&meta.RESTMapping{
			Scope: meta.RESTScopeRoot,
		}, nil)

	ctx := context.Background()
	v, err := ne.Check(ctx, owner, obj)
	require.NoError(t, err)
	assert.Equal(t, []Violation{
		{
			Position: "Hans /test",
			Error:    "Must be namespaced scoped when part of an non-cluster-scoped API.",
		},
	}, v)
}
