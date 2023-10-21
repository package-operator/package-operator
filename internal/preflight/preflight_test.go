package preflight

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestCheckAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner := &unstructured.Unstructured{}

	violations, err := CheckAll(
		ctx, List{}, owner,
		[]client.Object{
			&unstructured.Unstructured{},
		})
	require.NoError(t, err)
	assert.Empty(t, violations)
}

func TestCheckAllInPhase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner := &unstructured.Unstructured{}

	violations, err := CheckAllInPhase(
		ctx, List{}, owner,
		corev1alpha1.ObjectSetTemplatePhase{
			Objects: []corev1alpha1.ObjectSetObject{
				{Object: unstructured.Unstructured{}},
			},
		}, []unstructured.Unstructured{{}})
	require.NoError(t, err)
	assert.Empty(t, violations)
}

func Test_addPositionToViolations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	obj := &unstructured.Unstructured{}
	obj.SetName("test")
	obj.SetNamespace("testns")
	obj.SetKind("Something")
	violations := []Violation{
		{},
	}

	addPositionToViolations(ctx, obj, &violations)

	assert.Equal(t, "Something testns/test", violations[0].Position)
}

func Test_addPositionToViolations_withPhase(t *testing.T) {
	t.Parallel()
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

	addPositionToViolations(ctx, obj, &violations)

	assert.Equal(t, `Phase "123", Something testns/test`,
		violations[0].Position)
}

func TestList(t *testing.T) {
	t.Parallel()
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
	require.NoError(t, err)
	assert.True(t, called, "must have been called")
}

func TestPreflightListOk(t *testing.T) {
	t.Parallel()

	var called1, called2 bool

	list := PhasesCheckerList{
		phasesCheckerFn(func(ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase) (violations []Violation, err error) {
			called1 = true
			return
		}),
		phasesCheckerFn(func(ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase) (violations []Violation, err error) {
			called2 = true
			return
		}),
	}

	phases := []corev1alpha1.ObjectSetTemplatePhase{
		{Name: "test"},
	}
	ctx := context.Background()

	_, err := list.Check(ctx, phases)
	require.NoError(t, err)
	assert.True(t, called1, "first checker must have been called")
	assert.True(t, called2, "second checker must have been called")
}

var errChecker = errors.New("checker error")

func TestPreflightListWithError(t *testing.T) {
	t.Parallel()
	var called1, called2, called3 bool

	list := PhasesCheckerList{
		phasesCheckerFn(func(ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase) (violations []Violation, err error) {
			called1 = true
			return
		}),
		phasesCheckerFn(func(ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase) (violations []Violation, err error) {
			called2 = true
			return violations, errChecker
		}),
		phasesCheckerFn(func(ctx context.Context, phases []corev1alpha1.ObjectSetTemplatePhase) (violations []Violation, err error) {
			called3 = true
			return
		}),
	}

	phases := []corev1alpha1.ObjectSetTemplatePhase{
		{Name: "test"},
	}
	ctx := context.Background()

	_, err := list.Check(ctx, phases)
	require.ErrorIs(t, err, errChecker)
	assert.True(t, called1, "first checker must have been called")
	assert.True(t, called2, "second checker must have been called")
	assert.False(t, called3, "third checker must not have been called")
}
