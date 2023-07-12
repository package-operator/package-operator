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
	pkg.UpdatePhase()
	p := pkg.ClientObject().(*corev1alpha1.Package)
	assert.Equal(
		t, corev1alpha1.PackagePhaseUnpacking, p.Status.Phase)

	p.Spec.Image = "test"
	assert.Equal(t, p.Spec.Image, pkg.GetImage())

	pkg.SetUnpackedHash("123")
	assert.Equal(t, "123", p.Status.UnpackedHash)
	assert.Equal(t, "123", pkg.GetUnpackedHash())

	p.Spec.Config = &runtime.RawExtension{}
	tc := pkg.TemplateContext()
	assert.Same(t, p.Spec.Config, tc.Config)
}

func TestGenericClusterPackage(t *testing.T) {
	t.Parallel()
	pkg := NewGenericClusterPackage(testScheme)

	assert.NotNil(t, pkg.ClientObject())
	pkg.UpdatePhase()
	p := pkg.ClientObject().(*corev1alpha1.ClusterPackage)
	assert.Equal(
		t, corev1alpha1.PackagePhaseUnpacking, p.Status.Phase)

	p.Spec.Image = "test"
	assert.Equal(t, p.Spec.Image, pkg.GetImage())

	pkg.SetUnpackedHash("123")
	assert.Equal(t, "123", p.Status.UnpackedHash)
	assert.Equal(t, "123", pkg.GetUnpackedHash())

	p.Spec.Config = &runtime.RawExtension{}
	tc := pkg.TemplateContext()
	assert.Same(t, p.Spec.Config, tc.Config)
}

func Test_updatePackagePhase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		conditions []metav1.Condition
		expected   corev1alpha1.PackageStatusPhase
	}{
		{
			name: "Invalid",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.PackageInvalid,
					Status: metav1.ConditionTrue,
				},
			},
			expected: corev1alpha1.PackagePhaseInvalid,
		},
		{
			name:       "Unpacking",
			conditions: []metav1.Condition{},
			expected:   corev1alpha1.PackagePhaseUnpacking,
		},
		{
			name: "Progressing",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.PackageUnpacked,
					Status: metav1.ConditionTrue,
				},
				{
					Type:   corev1alpha1.PackageProgressing,
					Status: metav1.ConditionTrue,
				},
			},
			expected: corev1alpha1.PackagePhaseProgressing,
		},
		{
			name: "Available",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.PackageUnpacked,
					Status: metav1.ConditionTrue,
				},
				{
					Type:   corev1alpha1.PackageAvailable,
					Status: metav1.ConditionTrue,
				},
			},
			expected: corev1alpha1.PackagePhaseAvailable,
		},
		{
			name: "NotReady",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.PackageUnpacked,
					Status: metav1.ConditionTrue,
				},
			},
			expected: corev1alpha1.PackagePhaseNotReady,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pkg := &GenericPackage{
				Package: corev1alpha1.Package{
					Status: corev1alpha1.PackageStatus{
						Conditions: test.conditions,
					},
				},
			}
			updatePackagePhase(pkg)
			assert.Equal(t, test.expected, pkg.Package.Status.Phase)
		})
	}
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
