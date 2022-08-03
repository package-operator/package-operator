package ownerhandling

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"package-operator.run/package-operator/internal/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestSetControllerReference(t *testing.T) {
	s := &OwnerStrategyAnnotation{}
	cm1 := testutil.NewConfigMap()
	obj := testutil.NewSecret()
	scheme := testutil.NewTestSchemeWithCoreV1()

	err := s.SetControllerReference(cm1, obj, scheme)
	assert.NoError(t, err)

	ownerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.Equal(t, cm1.Name, ownerRefs[0].Name)
		assert.Equal(t, cm1.Namespace, ownerRefs[0].Namespace)
		assert.Equal(t, "ConfigMap", ownerRefs[0].Kind)
		assert.Equal(t, true, *ownerRefs[0].Controller)
	}

	cm2 := testutil.NewConfigMap()
	err = s.SetControllerReference(cm2, obj, scheme)
	assert.Error(t, err, controllerutil.AlreadyOwnedError{})

	s.ReleaseController(obj)

	err = s.SetControllerReference(cm2, obj, scheme)
	assert.NoError(t, err)
	assert.True(t, s.IsOwner(cm1, obj))
	assert.True(t, s.IsOwner(cm2, obj))
}

func TestOwnerStrategyAnnotation_ReleaseController(t *testing.T) {
	s := &OwnerStrategyAnnotation{}
	owner := testutil.NewConfigMap()
	obj := testutil.NewSecret()
	scheme := testutil.NewTestSchemeWithCoreV1()

	err := s.SetControllerReference(owner, obj, scheme)
	assert.NoError(t, err)

	ownerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.NotNil(t, ownerRefs[0].Controller)
	}

	s.ReleaseController(obj)
	ownerRefs = s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.Nil(t, ownerRefs[0].Controller)
	}
}

func newConfigMapAnnotationOwnerRef() annotationOwnerRef {
	cm := testutil.NewConfigMap()
	return annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID(rand.String(7)),
		Name:       cm.Name,
		Namespace:  cm.Namespace,
		Controller: pointer.BoolPtr(true),
	}
}

func TestOwnerStrategyAnnotation_IndexOf(t *testing.T) {
	ownerRef1 := newConfigMapAnnotationOwnerRef()
	ownerRef2 := newConfigMapAnnotationOwnerRef()
	ownerRef3 := newConfigMapAnnotationOwnerRef()

	s := &OwnerStrategyAnnotation{}
	i1 := s.indexOf([]annotationOwnerRef{ownerRef1, ownerRef2}, ownerRef1)
	assert.Equal(t, 0, i1)
	i2 := s.indexOf([]annotationOwnerRef{ownerRef1, ownerRef2}, ownerRef3)
	assert.Equal(t, -1, i2)
}

func TestOwnerStrategyAnnotation_setOwnerReferences(t *testing.T) {
	ownerRef := newConfigMapAnnotationOwnerRef()
	obj := testutil.NewSecret()

	s := &OwnerStrategyAnnotation{}
	s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
	gottenOwnerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, gottenOwnerRefs, 1) {
		assert.Equal(t, gottenOwnerRefs[0], ownerRef)
	}
}

func TestAnnotationEnqueueOwnerHandler_GetOwnerReconcileRequest(t *testing.T) {
	ownerRef := newConfigMapAnnotationOwnerRef()
	s := &OwnerStrategyAnnotation{}
	obj := testutil.NewSecret()
	s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
	h := AnnotationEnqueueOwnerHandler{
		OwnerType:    &corev1.ConfigMap{},
		IsController: true,
		ownerGK: schema.GroupKind{
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
