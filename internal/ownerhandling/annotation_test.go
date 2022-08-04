package ownerhandling

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "cmtestns",
			UID:       types.UID("1234"),
		},
	}
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

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm2",
			Namespace: "cmtestns",
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

func TestOwnerStrategyAnnotation_IndexOf(t *testing.T) {
	ownerRef1 := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("cm1___3245"),
		Name:       "cm1",
		Namespace:  "cm1namespace",
		Controller: pointer.Bool(false),
	}
	ownerRef2 := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("cm2___4326"),
		Name:       "cm2",
		Namespace:  "cm2namespace",
		Controller: pointer.Bool(false),
	}
	ownerRef3 := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("cm3___43643"),
		Name:       "cm3",
		Namespace:  "cm3namespace",
		Controller: pointer.Bool(false),
	}

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
	tests := []struct {
		name              string
		isOwnerController *bool
		enqueue           AnnotationEnqueueRequestForOwner
		requestExpected   bool
	}{
		{
			name:              "owner is controller, enqueue is controller, types match",
			isOwnerController: pointer.Bool(true),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: true,
			},
			requestExpected: true,
		},
		{
			name:              "owner is not controller, enqueue is controller, types match",
			isOwnerController: pointer.Bool(false),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: true,
			},
			requestExpected: false,
		},
		{
			name:              "owner is controller, enqueue is not controller, types match",
			isOwnerController: pointer.Bool(true),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: false,
			},
			requestExpected: true,
		},
		{
			name:              "owner is not controller, enqueue is not controller, types match",
			isOwnerController: pointer.Bool(false),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: false,
			},
			requestExpected: true,
		},
		{
			name:              "owner controller is nil, enqueue is controller, types match",
			isOwnerController: nil,
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: true,
			},
			requestExpected: false,
		},
		{
			name:              "owner controller is nil, enqueue is not controller, types match",
			isOwnerController: nil,
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: false,
			},
			requestExpected: true,
		},
		{
			name:              "owner is controller, enqueue is controller, types do not match",
			isOwnerController: pointer.Bool(true),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &appsv1.Deployment{},
				IsController: true,
			},
			requestExpected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ownerRef := newConfigMapAnnotationOwnerRef()
			ownerRef.Controller = test.isOwnerController
			s := &OwnerStrategyAnnotation{}
			obj := testutil.NewSecret()
			s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
			scheme := testutil.NewTestSchemeWithCoreV1AppsV1()
			err := test.enqueue.parseOwnerTypeGroupKind(scheme)
			require.NoError(t, err)
			r := test.enqueue.getOwnerReconcileRequest(obj)
			if test.requestExpected {
				assert.Equal(t, r, []reconcile.Request{
					{
						NamespacedName: client.ObjectKey{
							Name:      ownerRef.Name,
							Namespace: ownerRef.Namespace,
						},
					},
				})
			} else {
				assert.Len(t, r, 0)
			}

		})
	}
}

func TestAnnotationEnqueueOwnerHandler_ParseOwnerTypeGroupKind(t *testing.T) {
	h := &AnnotationEnqueueRequestForOwner{
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

func newConfigMapAnnotationOwnerRef() annotationOwnerRef {
	ownerRef1 := annotationOwnerRef{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		UID:        types.UID("cm1___3245"),
		Name:       "cm1",
		Namespace:  "cm1namespace",
		Controller: pointer.Bool(false),
	}
	return ownerRef1
}
