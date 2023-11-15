package preflight

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectDuplicate(t *testing.T) {
	t.Parallel()

	obj := corev1alpha1.ObjectSetObject{}
	obj.Object.SetName("test")
	obj.Object.SetNamespace("test-ns")
	obj.Object.SetKind("Hans")

	phases := []corev1alpha1.ObjectSetTemplatePhase{
		{Name: "phase1", Objects: []corev1alpha1.ObjectSetObject{obj}},
		{Name: "phase2", Objects: []corev1alpha1.ObjectSetObject{obj}},
	}

	od := NewObjectDuplicate()

	ctx := context.Background()
	v, err := od.Check(ctx, phases)
	require.NoError(t, err)
	assert.Len(t, v, 1)
	assert.Equal(t, "Phase \"phase2\", Hans test-ns/test: Duplicate Object", v[0].String())
}
