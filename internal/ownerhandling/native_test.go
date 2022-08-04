package ownerhandling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"package-operator.run/package-operator/internal/testutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestOwnerStrategyNative_SetControllerReference(t *testing.T) {
	s := &OwnerStrategyNative{}
	obj := testutil.NewSecret()
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: obj.Namespace,
			UID:       types.UID("1234"),
		},
	}
	scheme := testutil.NewTestSchemeWithCoreV1()

	err := s.SetControllerReference(cm1, obj, scheme)
	assert.NoError(t, err)

	ownerRefs := obj.GetOwnerReferences()
	if assert.Len(t, ownerRefs, 1) {
		assert.Equal(t, cm1.Name, ownerRefs[0].Name)
		assert.Equal(t, "ConfigMap", ownerRefs[0].Kind)
		assert.Equal(t, true, *ownerRefs[0].Controller)
	}

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm2",
			Namespace: obj.Namespace,
			UID:       types.UID("56789"),
		},
	}
	err = s.SetControllerReference(cm2, obj, scheme)
	assert.Error(t, err, controllerutil.AlreadyOwnedError{})

	s.ReleaseController(obj)

	err = s.SetControllerReference(cm2, obj, scheme)
	assert.NoError(t, err)
	assert.True(t, s.IsOwner(cm1, obj))
	assert.True(t, s.IsOwner(cm2, obj))
}

func TestOwnerStrategyNative_IsOwner(t *testing.T) {
	s := &OwnerStrategyNative{}
	obj := testutil.NewSecret()
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: obj.Namespace,
			UID:       types.UID("1234"),
		},
	}
	scheme := testutil.NewTestSchemeWithCoreV1()

	err := s.SetControllerReference(cm1, obj, scheme)
	require.NoError(t, err)

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm2",
			Namespace: obj.Namespace,
			UID:       types.UID("56789"),
		},
	}
	assert.True(t, s.IsOwner(cm1, obj))
	assert.False(t, s.IsOwner(cm2, obj))
}

func TestOwnerStrategyNative_ReleaseController(t *testing.T) {
	s := &OwnerStrategyNative{}
	obj := testutil.NewSecret()
	owner := testutil.NewConfigMap()
	owner.Namespace = obj.Namespace
	scheme := testutil.NewTestSchemeWithCoreV1()

	err := s.SetControllerReference(owner, obj, scheme)
	assert.NoError(t, err)

	ownerRefs := obj.GetOwnerReferences()
	if assert.Len(t, ownerRefs, 1) {
		assert.NotNil(t, ownerRefs[0].Controller)
	}

	s.ReleaseController(obj)
	ownerRefs = obj.GetOwnerReferences()
	if assert.Len(t, ownerRefs, 1) {
		assert.Nil(t, ownerRefs[0].Controller)
	}
}
