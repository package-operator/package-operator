package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestGenericPackage(t *testing.T) {
	t.Parallel()
	pkg := NewGenericPackage(testScheme)

	assert.NotNil(t, pkg.ClientObject())
	p := pkg.ClientObject().(*corev1alpha1.Package)

	p.Spec.Image = "test"
	assert.Equal(t, p.Spec.Image, pkg.GetSpecImage())

	pkg.SetStatusUnpackedHash("123")
	assert.Equal(t, "123", p.Status.UnpackedHash)
	assert.Equal(t, "123", pkg.GetStatusUnpackedHash())

	p.Spec.Config = &runtime.RawExtension{}
	tc := pkg.GetSpecTemplateContext()
	assert.Same(t, p.Spec.Config, tc.Config)

	assert.Empty(t, pkg.GetSpecComponent())
	p.Spec.Component = "test_component"
	assert.Equal(t, p.Spec.Component, pkg.GetSpecComponent())

	assert.Empty(t, pkg.GetStatusConditions())
	p.Status.Conditions = []metav1.Condition{
		{
			ObservedGeneration: 1,
			Type:               "test-type",
			Reason:             "test-reason",
			Message:            "test-message",
		},
	}
	assert.Equal(t, p.Status.Conditions, *pkg.GetStatusConditions())

	p.Status.Revision = int64(2)
	assert.Equal(t, p.Status.Revision, pkg.GetStatusRevision())

	pkg.SetSpecPaused(true)
	assert.True(t, pkg.GetSpecPaused())
	pkg.SetSpecPaused(false)
	assert.False(t, pkg.GetSpecPaused())
}

func TestGenericClusterPackage(t *testing.T) {
	t.Parallel()
	pkg := NewGenericClusterPackage(testScheme)

	assert.NotNil(t, pkg.ClientObject())
	p := pkg.ClientObject().(*corev1alpha1.ClusterPackage)

	p.Spec.Image = "test"
	assert.Equal(t, p.Spec.Image, pkg.GetSpecImage())

	pkg.SetStatusUnpackedHash("123")
	assert.Equal(t, "123", p.Status.UnpackedHash)
	assert.Equal(t, "123", pkg.GetStatusUnpackedHash())

	p.Spec.Config = &runtime.RawExtension{}
	tc := pkg.GetSpecTemplateContext()
	assert.Same(t, p.Spec.Config, tc.Config)

	assert.Empty(t, pkg.GetSpecComponent())
	p.Spec.Component = "test_component"
	assert.Equal(t, p.Spec.Component, pkg.GetSpecComponent())

	assert.Empty(t, pkg.GetStatusConditions())
	p.Status.Conditions = []metav1.Condition{
		{
			ObservedGeneration: 1,
			Type:               "test-type",
			Reason:             "test-reason",
			Message:            "test-message",
		},
	}
	assert.Equal(t, p.Status.Conditions, *pkg.GetStatusConditions())

	p.Status.Revision = int64(2)
	assert.Equal(t, p.Status.Revision, pkg.GetStatusRevision())

	pkg.SetSpecPaused(true)
	assert.True(t, pkg.GetSpecPaused())
	pkg.SetSpecPaused(false)
	assert.False(t, pkg.GetSpecPaused())
}

func Test_templateContextObjectMetaFromObjectMeta(t *testing.T) {
	t.Parallel()
	var (
		name        = "test"
		namespace   = "testns"
		labels      = map[string]string{"ltest": "ltest"}
		annotations = map[string]string{"atest": "atest"}
	)
	tcom := templateContextObjectMetaFromObjectMeta(metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	})

	assert.Equal(t, name, tcom.Name)
	assert.Equal(t, namespace, tcom.Namespace)
	assert.Equal(t, labels, tcom.Labels)
	assert.Equal(t, annotations, tcom.Annotations)
}
