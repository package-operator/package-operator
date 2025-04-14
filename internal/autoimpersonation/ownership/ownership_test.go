package ownership

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"package-operator.run/apis"
	"package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers/objecttemplate"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := apis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestVerifyOwnership_UnsupportedKind(t *testing.T) {
	t.Parallel()

	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
	}

	_, err := VerifyOwnership(&cm, &cm)
	assert.ErrorIs(t, err, ErrUnsupportedOwnerKind)
}

func TestVerifyOwnership_Package(t *testing.T) {
	t.Parallel()

	pkg := adapters.GenericPackage{
		Package: v1alpha1.Package{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Package",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-package",
				Namespace: "test-package-ns",
			},
			Spec:   v1alpha1.PackageSpec{},
			Status: v1alpha1.PackageStatus{},
		},
	}

	objectDeployment := adapters.ObjectDeployment{
		ObjectDeployment: v1alpha1.ObjectDeployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectDeployment",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "different-package",
				Namespace: "test-package-ns",
			},
			Spec:   v1alpha1.ObjectDeploymentSpec{},
			Status: v1alpha1.ObjectDeploymentStatus{},
		},
	}

	isOwner, err := VerifyOwnership(objectDeployment.ClientObject(), pkg.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	objectDeployment.ClientObject().SetName("test-package")
	isOwner, err = VerifyOwnership(objectDeployment.ClientObject(), pkg.ClientObject())
	require.NoError(t, err)
	assert.True(t, isOwner)
}

func TestVerifyOwnership_ClusterPackage(t *testing.T) {
	t.Parallel()

	pkg := adapters.GenericClusterPackage{
		ClusterPackage: v1alpha1.ClusterPackage{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterPackage",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-package",
			},
			Spec:   v1alpha1.PackageSpec{},
			Status: v1alpha1.PackageStatus{},
		},
	}

	objectDeployment := adapters.ClusterObjectDeployment{
		ClusterObjectDeployment: v1alpha1.ClusterObjectDeployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterObjectDeployment",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "different-package",
			},
			Spec:   v1alpha1.ClusterObjectDeploymentSpec{},
			Status: v1alpha1.ClusterObjectDeploymentStatus{},
		},
	}

	isOwner, err := VerifyOwnership(objectDeployment.ClientObject(), pkg.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	objectDeployment.ClientObject().SetName("test-package")
	isOwner, err = VerifyOwnership(objectDeployment.ClientObject(), pkg.ClientObject())
	require.NoError(t, err)
	assert.True(t, isOwner)
}

func TestVerifyOwnership_ObjectTemplate(t *testing.T) {
	t.Parallel()

	objectTemplate := objecttemplate.GenericObjectTemplate{
		ObjectTemplate: v1alpha1.ObjectTemplate{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectTemplate",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-template",
				Namespace: "test-template-ns",
				UID:       "test-template-uid",
			},
			Spec:   v1alpha1.ObjectTemplateSpec{},
			Status: v1alpha1.ObjectTemplateStatus{},
		},
	}

	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-template-ns",
			UID:       "test-cm-uid",
		},
	}

	// ObjectTemplate and ConfigMap are not related
	isOwner, err := VerifyOwnership(&cm, objectTemplate.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	cm.SetOwnerReferences(newOwnerReferences(objectTemplate.ClientObject()))

	// Modifying ConfigMap ownerReferences does not pass verification
	isOwner, err = VerifyOwnership(&cm, objectTemplate.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	objectTemplate.SetStatusControllerOf(newControlledObjectReference(&cm))

	// Two-way ownership established
	isOwner, err = VerifyOwnership(&cm, objectTemplate.ClientObject())
	require.NoError(t, err)
	assert.True(t, isOwner)
}

func TestVerifyOwnership_ObjectDeployment(t *testing.T) {
	t.Parallel()

	objectDeployment := adapters.ObjectDeployment{
		ObjectDeployment: v1alpha1.ObjectDeployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectDeployment",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-od",
				Namespace: "test-od-ns",
				UID:       "test-od-uid",
			},
		},
	}

	objectSet := adapters.ObjectSetAdapter{
		ObjectSet: v1alpha1.ObjectSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectSet",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-os",
				Namespace: "test-od-ns",
				UID:       "test-os-uid",
			},
		},
	}

	isOwner, err := VerifyOwnership(objectSet.ClientObject(), objectDeployment.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	// one-way ownership
	objectSet.SetOwnerReferences(newOwnerReferences(objectDeployment.ClientObject()))

	isOwner, err = VerifyOwnership(objectSet.ClientObject(), objectDeployment.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	// two-way ownership
	objectDeployment.SetStatusControllerOf([]v1alpha1.ControlledObjectReference{
		newControlledObjectReference(objectSet.ClientObject()),
	})

	isOwner, err = VerifyOwnership(objectSet.ClientObject(), objectDeployment.ClientObject())
	require.NoError(t, err)
	assert.True(t, isOwner)

	// multiple objectSets
	otherObjectSet := adapters.ObjectSetAdapter{
		ObjectSet: v1alpha1.ObjectSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectSet",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-test-os",
				Namespace: "test-od-ns",
				UID:       "other-test-os-uid",
			},
		},
	}
	objectDeployment.SetStatusControllerOf([]v1alpha1.ControlledObjectReference{
		newControlledObjectReference(objectSet.ClientObject()),
		newControlledObjectReference(otherObjectSet.ClientObject()),
	})
	otherObjectSet.SetOwnerReferences(newOwnerReferences(objectDeployment.ClientObject()))

	for _, os := range []adapters.ObjectSetAdapter{objectSet, otherObjectSet} {
		isOwner, err = VerifyOwnership(os.ClientObject(), objectDeployment.ClientObject())
		require.NoError(t, err)
		assert.True(t, isOwner)
	}
}

func TestVerifyOwnership_ObjectSet(t *testing.T) {
	t.Parallel()

	objectSet := adapters.ObjectSetAdapter{
		ObjectSet: v1alpha1.ObjectSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectSet",
				APIVersion: "package-operator.run/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-os",
				Namespace: "test-os-ns",
				UID:       "test-os-uid",
			},
		},
	}

	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-os-ns",
			UID:       "test-cm-uid",
		},
	}

	isOwner, err := VerifyOwnership(&cm, objectSet.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	// one-way ownership
	cm.SetOwnerReferences(newOwnerReferences(objectSet.ClientObject()))

	isOwner, err = VerifyOwnership(&cm, objectSet.ClientObject())
	require.NoError(t, err)
	assert.False(t, isOwner)

	// two-way ownership
	objectSet.SetStatusControllerOf([]v1alpha1.ControlledObjectReference{
		newControlledObjectReference(&cm),
	})

	isOwner, err = VerifyOwnership(&cm, objectSet.ClientObject())
	require.NoError(t, err)
	assert.True(t, isOwner)
}

func newOwnerReferences(o client.Object) []metav1.OwnerReference {
	t := true
	APIVersion, kind := o.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return []metav1.OwnerReference{{
		APIVersion:         APIVersion,
		Kind:               kind,
		Name:               o.GetName(),
		UID:                o.GetUID(),
		Controller:         &t,
		BlockOwnerDeletion: &t,
	}}
}

func newControlledObjectReference(o client.Object) v1alpha1.ControlledObjectReference {
	gvk := o.GetObjectKind().GroupVersionKind()
	return v1alpha1.ControlledObjectReference{
		Kind:      gvk.Kind,
		Group:     gvk.Group,
		Name:      o.GetName(),
		Namespace: o.GetNamespace(),
	}
}
