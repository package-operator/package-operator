package ownerhandling

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestIndexOf(t *testing.T) {
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

func TestGetControllerReference(t *testing.T) {
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

func TestGetOwnerReconcileRequest(t *testing.T) {
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
