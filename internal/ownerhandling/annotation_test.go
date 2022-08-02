package ownerhandling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestSetControllerReference(t *testing.T) {
	s := &OwnerStrategyAnnotation{}
	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cmtest",
			Namespace: "cmtestns",
		},
	}
	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
			UID:       types.UID("1234"),
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	err := s.SetControllerReference(owner, obj, scheme)
	assert.NoError(t, err)
	ownerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.Equal(t, owner.Name, ownerRefs[0].Name)
		assert.Equal(t, owner.Namespace, ownerRefs[0].Namespace)
		assert.Equal(t, "ConfigMap", ownerRefs[0].Kind)
	}
	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cmtest2",
			Namespace: "cmtestns2",
			UID:       types.UID("5678"),
		},
	}
	err = s.SetControllerReference(cm2, obj, scheme)
	assert.Error(t, err, controllerutil.AlreadyOwnedError{})

	s.ReleaseController(obj)

	err = s.SetControllerReference(cm2, obj, scheme)
	assert.NoError(t, err)
	assert.True(t, s.IsOwner(owner, obj))
	assert.True(t, s.IsOwner(cm2, obj))
}

func TestOwnerStrategyAnnotation_ReleaseController(t *testing.T) {
	s := &OwnerStrategyAnnotation{}
	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cmtest",
			Namespace: "cmtestns",
		},
	}
	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
			UID:       types.UID("1234"),
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	err := s.SetControllerReference(owner, obj, scheme)
	assert.NoError(t, err)
	ownerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.Equal(t, owner.Name, ownerRefs[0].Name)
		assert.Equal(t, owner.Namespace, ownerRefs[0].Namespace)
		assert.Equal(t, "ConfigMap", ownerRefs[0].Kind)
	}

	s.ReleaseController(obj)
	ownerRefs = s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.Nil(t, ownerRefs[0].Controller)
	}
}

func TestOwnerStrategyAnnotation_IndexOf(t *testing.T) {
	ownerRef1 := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("1234"),
		Name:       "cmtest1",
		Namespace:  "cmtestns",
		Controller: pointer.BoolPtr(true),
	}
	ownerRef2 := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("6789"),
		Name:       "cmtest2",
		Namespace:  "cmtestns",
		Controller: pointer.BoolPtr(true),
	}
	ownerRef3 := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("1919"),
		Name:       "cmtest2",
		Namespace:  "cmtestns",
		Controller: pointer.BoolPtr(true),
	}
	s := &OwnerStrategyAnnotation{}
	i1 := s.indexOf([]annotationOwnerRef{ownerRef1, ownerRef2}, ownerRef1)
	assert.Equal(t, 0, i1)
	i2 := s.indexOf([]annotationOwnerRef{ownerRef1, ownerRef2}, ownerRef3)
	assert.Equal(t, -1, i2)
}

func TestOwnerStrategyAnnotation_setOwnerReferences(t *testing.T) {
	ownerRef := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("1234"),
		Name:       "cmtest",
		Namespace:  "cmtestns",
		Controller: pointer.BoolPtr(true),
	}
	s := &OwnerStrategyAnnotation{}
	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
		},
	}
	s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
	gottenOwnerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, gottenOwnerRefs, 1) {
		assert.Equal(t, gottenOwnerRefs[0], ownerRef)
	}
}

func TestAnnotationEnqueueOwnerHandler_GetOwnerReconcileRequest(t *testing.T) {
	ownerRef := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("1234"),
		Name:       "cmtest",
		Namespace:  "cmtestns",
		Controller: pointer.BoolPtr(true),
	}
	s := &OwnerStrategyAnnotation{}
	obj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "testns",
		},
	}
	s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
	// TODO: PUT IN FIELD NAMES
	h := AnnotationEnqueueOwnerHandler{
		&corev1.ConfigMap{},
		true,
		schema.GroupKind{
			Kind: "ConfigMap",
		},
	}
	r := h.getOwnerReconcileRequest(obj)
	if assert.Len(t, r, 1) {
		assert.Equal(t, r, []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Name:      ownerRef.Name,
					Namespace: ownerRef.Namespace,
				},
			},
		},
		)
	}
}

func TestAnnotationEnqueueOwnerHandler_ParseOwnerTypeGroupKind(t *testing.T) {
	h := &AnnotationEnqueueOwnerHandler{
		OwnerType:    &appsv1.Deployment{},
		IsController: true,
	}

	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	err := h.parseOwnerTypeGroupKind(scheme)
	assert.NoError(t, err)
	expectedGK := schema.GroupKind{
		Group: "apps",
		Kind:  "Deployment",
	}
	assert.Equal(t, expectedGK, h.ownerGK)
}
