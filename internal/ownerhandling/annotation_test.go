package ownerhandling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

func TestOwnerStrategyAnnotation_HasController(t *testing.T) {
	tests := []struct {
		name     string
		obj      metav1.Object
		expected bool
	}{
		{
			name: "controller in annotation",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm1",
					Namespace: "test",
					UID:       types.UID("1234"),
					Annotations: map[string]string{
						ownerStrategyAnnotationKey: `[{"uid":"123456", "kind":"ConfigMap", "name":"cm1", "controller":true}]`,
					},
				},
			},
			expected: true,
		},
		{
			name: "controller in ownerReferences",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm1",
					Namespace: "test",
					UID:       types.UID("1234"),
					OwnerReferences: []metav1.OwnerReference{
						{},
						{Controller: ptr.To(true)},
					},
				},
			},
			expected: true,
		},
		{
			name: "none",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm1",
					Namespace: "test",
					UID:       types.UID("1234"),
				},
			},
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := NewAnnotation(testScheme)
			r := s.HasController(test.obj)
			assert.Equal(t, test.expected, r)
		})
	}
}

func TestOwnerStrategyAnnotation_RemoveOwner(t *testing.T) {
	t.Parallel()
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "test",
			UID:       types.UID("1234"),
			Annotations: map[string]string{
				ownerStrategyAnnotationKey: `[{"uid":"123456", "kind":"ConfigMap", "name":"cm1"}]`,
			},
		},
	}
	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: obj.Namespace,
			UID:       types.UID("123456"),
		},
	}

	s := NewAnnotation(testScheme)
	s.RemoveOwner(owner, obj)

	assert.Equal(t, `[]`, obj.Annotations[ownerStrategyAnnotationKey])
}

func TestOwnerStrategyAnnotation_SetOwnerReference(t *testing.T) {
	t.Parallel()
	s := NewAnnotation(testScheme)
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "cmtestns",
			UID:       types.UID("1234"),
		},
	}
	obj := testutil.NewSecret()

	require.NoError(t, s.SetOwnerReference(cm1, obj))

	ownerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.Equal(t, cm1.Name, ownerRefs[0].Name)
		assert.Equal(t, cm1.Namespace, ownerRefs[0].Namespace)
		assert.Equal(t, "ConfigMap", ownerRefs[0].Kind)
		assert.Nil(t, ownerRefs[0].Controller)
	}

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm2",
			Namespace: "cmtestns",
			UID:       types.UID("56789"),
		},
	}
	require.NoError(t, s.SetOwnerReference(cm2, obj))
}

func TestOwnerStrategyAnnotation_SetControllerReference(t *testing.T) {
	t.Parallel()
	s := NewAnnotation(testScheme)
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "cmtestns",
			UID:       types.UID("1234"),
		},
	}
	obj := testutil.NewSecret()

	err := s.SetControllerReference(cm1, obj)
	require.NoError(t, err)

	ownerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, ownerRefs, 1) {
		assert.Equal(t, cm1.Name, ownerRefs[0].Name)
		assert.Equal(t, cm1.Namespace, ownerRefs[0].Namespace)
		assert.Equal(t, "ConfigMap", ownerRefs[0].Kind)
		assert.True(t, *ownerRefs[0].Controller)
	}

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm2",
			Namespace: "cmtestns",
			UID:       types.UID("56789"),
		},
	}
	err = s.SetControllerReference(cm2, obj)
	require.Error(t, err, controllerutil.AlreadyOwnedError{})

	s.ReleaseController(obj)

	err = s.SetControllerReference(cm2, obj)
	require.NoError(t, err)
	assert.True(t, s.IsOwner(cm1, obj))
	assert.True(t, s.IsOwner(cm2, obj))
}

func TestOwnerStrategyAnnotation_ReleaseController(t *testing.T) {
	t.Parallel()
	s := NewAnnotation(testScheme)
	owner := testutil.NewConfigMap()
	obj := testutil.NewSecret()

	err := s.SetControllerReference(owner, obj)
	require.NoError(t, err)

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
	t.Parallel()
	testCases := []struct {
		name          string
		ownerRef      annotationOwnerRef
		ownerRefs     []annotationOwnerRef
		expectedIndex int
	}{
		{
			name: "owner references are not defined",
			ownerRef: annotationOwnerRef{
				APIVersion: "test/v1",
				Kind:       "Testi",
				Name:       "cm1___3245",
			},
			ownerRefs:     []annotationOwnerRef{},
			expectedIndex: -1,
		},
		{
			name:     "owner reference is not defined",
			ownerRef: annotationOwnerRef{},
			ownerRefs: []annotationOwnerRef{
				{
					APIVersion: "test/v1",
					Kind:       "Testi",
					Name:       "cm1___3245",
				},
			},
			expectedIndex: -1,
		},
		{
			name:          "owner reference and references are not defined",
			ownerRef:      annotationOwnerRef{},
			ownerRefs:     []annotationOwnerRef{},
			expectedIndex: -1,
		},
		{
			name: "owner reference is not present in references",
			ownerRef: annotationOwnerRef{
				APIVersion: "test/v1",
				Kind:       "Testi",
				Name:       "cm1___3245",
			},
			ownerRefs: []annotationOwnerRef{
				{
					APIVersion: "test/v1",
					Kind:       "Testi",
					Name:       "cm1___3265",
				},
				{
					APIVersion: "test/v1",
					Kind:       "Testi",
					Name:       "cm1___3456",
				},
			},
			expectedIndex: -1,
		},
		{
			name: "owner reference is present in references",
			ownerRef: annotationOwnerRef{
				APIVersion: "test/v1",
				Kind:       "Testi",
				Name:       "cm1___3245",
			},
			ownerRefs: []annotationOwnerRef{
				{
					APIVersion: "test/v1",
					Kind:       "Testi",
					Name:       "cm1___3265",
				},
				{
					APIVersion: "test/v1",
					Kind:       "Testi",
					Name:       "cm1___3456",
				},
				{
					APIVersion: "test/v1",
					Kind:       "Testi",
					Name:       "cm1___3245",
				},
			},
			expectedIndex: 2,
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := OwnerStrategyAnnotation{}
			resultIndex := s.indexOf(tc.ownerRefs, tc.ownerRef)
			assert.Equal(t, tc.expectedIndex, resultIndex)
		})
	}
}

func TestOwnerStrategyAnnotation_setOwnerReferences(t *testing.T) {
	t.Parallel()
	ownerRef := newConfigMapAnnotationOwnerRef()
	obj := testutil.NewSecret()

	s := NewAnnotation(testScheme)
	s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
	gottenOwnerRefs := s.getOwnerReferences(obj)
	if assert.Len(t, gottenOwnerRefs, 1) {
		assert.Equal(t, gottenOwnerRefs[0], ownerRef)
	}
}

func TestAnnotationEnqueueOwnerHandler_GetOwnerReconcileRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		isOwnerController *bool
		enqueue           AnnotationEnqueueRequestForOwner
		requestExpected   bool
	}{
		{
			name:              "owner is controller, enqueue is controller, types match",
			isOwnerController: ptr.To(true),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: true,
			},
			requestExpected: true,
		},
		{
			name:              "owner is not controller, enqueue is controller, types match",
			isOwnerController: ptr.To(false),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: true,
			},
			requestExpected: false,
		},
		{
			name:              "owner is controller, enqueue is not controller, types match",
			isOwnerController: ptr.To(true),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &corev1.ConfigMap{},
				IsController: false,
			},
			requestExpected: true,
		},
		{
			name:              "owner is not controller, enqueue is not controller, types match",
			isOwnerController: ptr.To(false),
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
			isOwnerController: ptr.To(true),
			enqueue: AnnotationEnqueueRequestForOwner{
				OwnerType:    &appsv1.Deployment{},
				IsController: true,
			},
			requestExpected: false,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ownerRef := newConfigMapAnnotationOwnerRef()
			ownerRef.Controller = test.isOwnerController
			s := NewAnnotation(testScheme)
			obj := testutil.NewSecret()
			s.setOwnerReferences(obj, []annotationOwnerRef{ownerRef})
			scheme := testutil.NewTestSchemeWithCoreV1AppsV1()
			err := test.enqueue.parseOwnerTypeGroupKind(scheme)
			require.NoError(t, err)
			r := test.enqueue.getOwnerReconcileRequest(obj)
			if test.requestExpected {
				assert.Equal(t, []reconcile.Request{
					{
						NamespacedName: client.ObjectKey{
							Name:      ownerRef.Name,
							Namespace: ownerRef.Namespace,
						},
					},
				}, r)
			} else {
				assert.Empty(t, r)
			}
		})
	}
}

func TestAnnotationEnqueueOwnerHandler_ParseOwnerTypeGroupKind(t *testing.T) {
	t.Parallel()
	h := &AnnotationEnqueueRequestForOwner{
		OwnerType:    &appsv1.Deployment{},
		IsController: true,
	}

	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	err := h.parseOwnerTypeGroupKind(scheme)
	require.NoError(t, err)
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
		Controller: ptr.To(false),
	}
	return ownerRef1
}

func TestOwnerStrategyAnnotation_IsController(t *testing.T) {
	t.Parallel()
	s := NewAnnotation(testScheme)
	obj := testutil.NewSecret()
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: obj.Namespace,
			UID:       types.UID("1234"),
		},
	}

	err := s.SetControllerReference(cm1, obj)
	require.NoError(t, err)

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm2",
			Namespace: obj.Namespace,
			UID:       types.UID("56789"),
		},
	}
	assert.True(t, s.IsController(cm1, obj))
	assert.False(t, s.IsController(cm2, obj))
}

func TestIsOwner(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name          string
		owner         *corev1.ConfigMap
		obj           *corev1.Secret
		expectedOwner bool
	}{
		{
			name:          "owner reference is not present",
			owner:         testutil.NewConfigMap(),
			obj:           testutil.NewSecret(),
			expectedOwner: false,
		},
		{
			name:  "owner reference is present",
			owner: testutil.NewConfigMap(),
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: "cmtestns",
					UID:       types.UID("asdfjkl"),
					Annotations: map[string]string{
						ownerStrategyAnnotationKey: `[{"kind":"ConfigMap", "apiVersion":"v1", "name":"cm","namespace":"cmtestns"}]`,
					},
				},
			},
			expectedOwner: true,
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewAnnotation(testScheme)
			resultOwner := s.IsOwner(tc.owner, tc.obj)
			assert.Equal(t, tc.expectedOwner, resultOwner)
		})
	}
}

func TestIsController(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name               string
		annOwnerRef        annotationOwnerRef
		expectedController bool
	}{
		{
			name:               "annotation owner ref not defined",
			annOwnerRef:        annotationOwnerRef{},
			expectedController: false,
		},
		{
			name: "controller is null",
			annOwnerRef: annotationOwnerRef{
				Controller: nil,
			},
			expectedController: false,
		},
		{
			name: "controller is false",
			annOwnerRef: annotationOwnerRef{
				Controller: ptr.To(false),
			},
			expectedController: false,
		},
		{
			name: "conroller is defined and true",
			annOwnerRef: annotationOwnerRef{
				Controller: ptr.To(true),
			},
			expectedController: true,
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			annOwnerRef := tc.annOwnerRef
			resultController := annOwnerRef.isController()
			assert.Equal(t, tc.expectedController, resultController)
		})
	}
}

func TestOwnerStrategyAnnotation_OwnerPatch(t *testing.T) {
	t.Parallel()
	s := NewAnnotation(testScheme)
	obj := testutil.NewSecret()
	obj.Annotations = map[string]string{
		corev1alpha1.ObjectSetRevisionAnnotation: "3",
	}
	owner := testutil.NewConfigMap()
	owner.Namespace = obj.Namespace
	err := s.SetControllerReference(owner, obj)
	require.NoError(t, err)

	patch, err := s.OwnerPatch(obj)
	require.NoError(t, err)

	expected := `{"metadata":{"annotations":{"package-operator.run/owners":"[{\"apiVersion\":\"v1\",` +
		`\"kind\":\"ConfigMap\",\"name\":\"cm\",\"namespace\":\"testns\",\"uid\":\"asdfjkl\",` +
		`\"controller\":true}]","package-operator.run/revision":"3"}}}`
	assert.Equal(t, expected, string(patch))
}

func TestOwnerStrategyAnnotation_EnqueueRequestForOwner(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	s := NewAnnotation(scheme)
	require.NotPanics(t, func() {
		s.EnqueueRequestForOwner(&appsv1.Deployment{}, meta.RESTMapper(nil), true)
	})
}
