package boxcutterutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"pkg.package-operator.run/boxcutter/machinery"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/validation"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
)

// isControllerCheckerMock implements the isControllerChecker interface.
type isControllerCheckerMock struct {
	mock.Mock
}

func (m *isControllerCheckerMock) IsController(owner, obj metav1.Object) bool {
	args := m.Called(owner, obj)
	return args.Bool(0)
}

func TestTranslateCollisionProtection(t *testing.T) {
	t.Parallel()

	t.Run("translates CollisionProtectionNone", func(t *testing.T) {
		t.Parallel()

		result := TranslateCollisionProtection(corev1alpha1.CollisionProtectionNone)
		assert.Equal(t, types.WithCollisionProtection(types.CollisionProtectionNone), result)
	})

	t.Run("translates CollisionProtectionPrevent", func(t *testing.T) {
		t.Parallel()

		result := TranslateCollisionProtection(corev1alpha1.CollisionProtectionPrevent)
		assert.Equal(t, types.WithCollisionProtection(types.CollisionProtectionPrevent), result)
	})

	t.Run("translates CollisionProtectionIfNoController", func(t *testing.T) {
		t.Parallel()

		result := TranslateCollisionProtection(corev1alpha1.CollisionProtectionIfNoController)
		assert.Equal(t, types.WithCollisionProtection(types.CollisionProtectionIfNoController), result)
	})

	t.Run("panics on invalid value", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			TranslateCollisionProtection(corev1alpha1.CollisionProtection("invalid"))
		})
	})

	t.Run("panics on empty value", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			TranslateCollisionProtection(corev1alpha1.CollisionProtection(""))
		})
	})
}

func TestGetControllerOf(t *testing.T) {
	t.Parallel()

	t.Run("returns empty list when no objects", func(t *testing.T) {
		t.Parallel()

		ownerStrategy := &isControllerCheckerMock{}
		scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
		owner := adapters.NewObjectSet(scheme)
		owner.ClientObject().SetName("test-owner")

		phaseResult := &phaseResultMock{
			objects: []machinery.ObjectResult{},
		}

		result := GetControllerOf(ownerStrategy, owner.ClientObject(), phaseResult)

		assert.Empty(t, result)
	})

	t.Run("returns only controlled objects", func(t *testing.T) {
		t.Parallel()

		ownerStrategy := &isControllerCheckerMock{}
		scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
		owner := adapters.NewObjectSet(scheme)
		owner.ClientObject().SetName("test-owner")

		obj1 := &objectMock{
			name:      "controlled-1",
			namespace: "ns-1",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
		}
		obj2 := &objectMock{
			name:      "not-controlled",
			namespace: "ns-2",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
		}
		obj3 := &objectMock{
			name:      "controlled-2",
			namespace: "ns-3",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Secret",
			},
		}

		phaseResult := &phaseResultMock{
			objects: []machinery.ObjectResult{
				&objectResultMock{obj: obj1},
				&objectResultMock{obj: obj2},
				&objectResultMock{obj: obj3},
			},
		}

		// Mock IsController to return true for obj1 and obj3, false for obj2
		ownerStrategy.On("IsController", owner.ClientObject(), obj1).Return(true)
		ownerStrategy.On("IsController", owner.ClientObject(), obj2).Return(false)
		ownerStrategy.On("IsController", owner.ClientObject(), obj3).Return(true)

		result := GetControllerOf(ownerStrategy, owner.ClientObject(), phaseResult)

		require.Len(t, result, 2)
		assert.Equal(t, corev1alpha1.ControlledObjectReference{
			Kind:      "Deployment",
			Group:     "apps",
			Name:      "controlled-1",
			Namespace: "ns-1",
		}, result[0])
		assert.Equal(t, corev1alpha1.ControlledObjectReference{
			Kind:      "Secret",
			Group:     "",
			Name:      "controlled-2",
			Namespace: "ns-3",
		}, result[1])
	})

	t.Run("returns empty list when no objects are controlled", func(t *testing.T) {
		t.Parallel()

		ownerStrategy := &isControllerCheckerMock{}
		scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
		owner := adapters.NewObjectSet(scheme)
		owner.ClientObject().SetName("test-owner")

		obj1 := &objectMock{
			name:      "obj-1",
			namespace: "ns-1",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
		}

		phaseResult := &phaseResultMock{
			objects: []machinery.ObjectResult{
				&objectResultMock{obj: obj1},
			},
		}

		ownerStrategy.On("IsController", owner.ClientObject(), obj1).Return(false)

		result := GetControllerOf(ownerStrategy, owner.ClientObject(), phaseResult)

		assert.Empty(t, result)
	})

	t.Run("handles cluster-scoped resources", func(t *testing.T) {
		t.Parallel()

		ownerStrategy := &isControllerCheckerMock{}
		scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
		owner := adapters.NewObjectSet(scheme)
		owner.ClientObject().SetName("test-owner")

		obj1 := &objectMock{
			name:      "cluster-resource",
			namespace: "", // Cluster-scoped
			gvk: schema.GroupVersionKind{
				Group:   "rbac.authorization.k8s.io",
				Version: "v1",
				Kind:    "ClusterRole",
			},
		}

		phaseResult := &phaseResultMock{
			objects: []machinery.ObjectResult{
				&objectResultMock{obj: obj1},
			},
		}

		ownerStrategy.On("IsController", owner.ClientObject(), obj1).Return(true)

		result := GetControllerOf(ownerStrategy, owner.ClientObject(), phaseResult)

		require.Len(t, result, 1)
		assert.Equal(t, corev1alpha1.ControlledObjectReference{
			Kind:      "ClusterRole",
			Group:     "rbac.authorization.k8s.io",
			Name:      "cluster-resource",
			Namespace: "",
		}, result[0])
	})

	t.Run("handles multiple controlled objects", func(t *testing.T) {
		t.Parallel()

		ownerStrategy := &isControllerCheckerMock{}
		scheme := testutil.NewTestSchemeWithCoreV1Alpha1()
		owner := adapters.NewObjectSet(scheme)
		owner.ClientObject().SetName("test-owner")

		objects := make([]machinery.ObjectResult, 0, 10)
		for i := range 10 {
			obj := &objectMock{
				name:      fmt.Sprintf("obj-%d", i),
				namespace: "ns-1",
				gvk: schema.GroupVersionKind{
					Group:   "",
					Version: "v1",
					Kind:    "ConfigMap",
				},
			}
			objects = append(objects, &objectResultMock{obj: obj})
			ownerStrategy.On("IsController", owner.ClientObject(), obj).Return(true)
		}

		phaseResult := &phaseResultMock{
			objects: objects,
		}

		result := GetControllerOf(ownerStrategy, owner.ClientObject(), phaseResult)

		assert.Len(t, result, 10)
	})
}

// Mock implementations for testing

type phaseResultMock struct {
	mock.Mock
	objects []machinery.ObjectResult
}

func (m *phaseResultMock) GetObjects() []machinery.ObjectResult {
	return m.objects
}

func (m *phaseResultMock) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *phaseResultMock) IsComplete() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *phaseResultMock) String() string {
	args := m.Called()
	return args.String(0)
}

func (m *phaseResultMock) GetValidationError() *validation.PhaseValidationError {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*validation.PhaseValidationError)
}

func (m *phaseResultMock) HasProgressed() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *phaseResultMock) InTransition() bool {
	args := m.Called()
	return args.Bool(0)
}

type objectResultMock struct {
	mock.Mock
	obj machinery.Object
}

func (m *objectResultMock) Object() machinery.Object {
	return m.obj
}

func (m *objectResultMock) Action() machinery.Action {
	args := m.Called()
	return args.Get(0).(machinery.Action)
}

func (m *objectResultMock) ProbeResults() types.ProbeResultContainer {
	args := m.Called()
	if args.Get(0) == nil {
		return types.ProbeResultContainer{}
	}
	return args.Get(0).(types.ProbeResultContainer)
}

func (m *objectResultMock) String() string {
	args := m.Called()
	return args.String(0)
}

func (m *objectResultMock) IsComplete() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *objectResultMock) IsPaused() bool {
	args := m.Called()
	return args.Bool(0)
}

type objectMock struct {
	mock.Mock
	name         string
	namespace    string
	generateName string
	gvk          schema.GroupVersionKind
}

func (m *objectMock) GetName() string {
	return m.name
}

func (m *objectMock) SetName(name string) {
	m.name = name
}

func (m *objectMock) GetGenerateName() string {
	return m.generateName
}

func (m *objectMock) SetGenerateName(generateName string) {
	m.generateName = generateName
}

func (m *objectMock) GetNamespace() string {
	return m.namespace
}

func (m *objectMock) SetNamespace(namespace string) {
	m.namespace = namespace
}

func (m *objectMock) GetSelfLink() string {
	return ""
}

func (m *objectMock) SetSelfLink(string) {}

func (m *objectMock) GetObjectKind() schema.ObjectKind {
	return &objectKindMock{gvk: m.gvk}
}

func (m *objectMock) GetUID() k8stypes.UID {
	return "test-uid"
}

func (m *objectMock) SetUID(k8stypes.UID) {}

func (m *objectMock) GetResourceVersion() string {
	return "1"
}

func (m *objectMock) SetResourceVersion(string) {}

func (m *objectMock) GetGeneration() int64 {
	return 1
}

func (m *objectMock) SetGeneration(int64) {}

func (m *objectMock) GetCreationTimestamp() metav1.Time {
	return metav1.Time{}
}

func (m *objectMock) SetCreationTimestamp(metav1.Time) {}

func (m *objectMock) GetDeletionTimestamp() *metav1.Time {
	return nil
}

func (m *objectMock) SetDeletionTimestamp(*metav1.Time) {}

func (m *objectMock) GetDeletionGracePeriodSeconds() *int64 {
	return nil
}

func (m *objectMock) SetDeletionGracePeriodSeconds(*int64) {}

func (m *objectMock) GetLabels() map[string]string {
	return nil
}

func (m *objectMock) SetLabels(map[string]string) {}

func (m *objectMock) GetAnnotations() map[string]string {
	return nil
}

func (m *objectMock) SetAnnotations(map[string]string) {}

func (m *objectMock) GetFinalizers() []string {
	return nil
}

func (m *objectMock) SetFinalizers([]string) {}

func (m *objectMock) GetOwnerReferences() []metav1.OwnerReference {
	return nil
}

func (m *objectMock) SetOwnerReferences([]metav1.OwnerReference) {}

func (m *objectMock) GetManagedFields() []metav1.ManagedFieldsEntry {
	return nil
}

func (m *objectMock) SetManagedFields([]metav1.ManagedFieldsEntry) {}

func (m *objectMock) DeepCopyObject() runtime.Object {
	return m
}

type objectKindMock struct {
	gvk schema.GroupVersionKind
}

func (m *objectKindMock) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	m.gvk = gvk
}

func (m *objectKindMock) GroupVersionKind() schema.GroupVersionKind {
	return m.gvk
}
