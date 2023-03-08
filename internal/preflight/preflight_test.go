package preflight

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestCheckAll(t *testing.T) {
	ctx := context.Background()
	owner := &unstructured.Unstructured{}

	violations, err := CheckAll(
		ctx, List{}, owner,
		[]client.Object{
			&unstructured.Unstructured{},
		})
	require.NoError(t, err)
	assert.Len(t, violations, 0)
}

func TestCheckAllInPhase(t *testing.T) {
	ctx := context.Background()
	owner := &unstructured.Unstructured{}

	violations, err := CheckAllInPhase(
		ctx, List{}, owner,
		corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{Object: unstructured.Unstructured{}},
			},
		})
	require.NoError(t, err)
	assert.Len(t, violations, 0)
}

func Test_addPositionToViolations(t *testing.T) {
	ctx := context.Background()
	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("testns")
	obj.SetKind("Something")
	violations := []Violation{
		{},
	}

	addPositionToViolations(ctx, obj, violations)

	assert.Equal(t, "Something testns/test", violations[0].Position)
}

func Test_addPositionToViolations_withPhase(t *testing.T) {
	ctx := context.Background()
	ctx = NewContextWithPhase(ctx, corev1alpha1.ObjectSetTemplatePhase{
		Name: "123",
	})

	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("testns")
	obj.SetKind("Something")
	violations := []Violation{
		{},
	}

	addPositionToViolations(ctx, obj, violations)

	assert.Equal(t, `Phase "123", Something testns/test`,
		violations[0].Position)
}

func TestList(t *testing.T) {
	var called bool
	list := List{
		CheckerFn(func(ctx context.Context, owner, obj client.Object) (violations []Violation, err error) {
			called = true
			return
		}),
	}

	owner := &unstructured.Unstructured{}
	obj := &unstructured.Unstructured{}
	ctx := context.Background()

	_, err := list.Check(ctx, owner, obj)
	assert.NoError(t, err)
	assert.True(t, called, "must have been called")
}
