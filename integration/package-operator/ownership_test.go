//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/autoimpersonation/ownership"
)

var typeMetaType = reflect.TypeOf(metav1.TypeMeta{})

// Test helper that uses reflection to get to the underlying struct value of a `runtime.Object`
// and set its TypeMeta field with data acquired from the passed scheme.
// Trivia:
// `runtime.Object` is a narrower `client.Object`, so this will work on `client.Object`s, too.
func requireSetTypeMeta(t *testing.T, o runtime.Object, scheme *runtime.Scheme) {
	t.Helper()

	// Get value from interface.
	value := reflect.ValueOf(o)

	// Dereference pointers.
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	// Ensure that a field called "TypeMeta" exists
	// and that it has the correct type.
	fieldType, fieldFound := value.Type().FieldByName("TypeMeta")
	require.True(t, fieldFound, "field is missing on input value")
	require.True(t, typeMetaType.AssignableTo(fieldType.Type),
		"field is having wrong type",
		"expected", typeMetaType,
		"actual", fieldType.Type)

	// Prepare TypeMeta value.
	gvk, err := apiutil.GVKForObject(o.(client.Object), scheme)
	require.NoError(t, err)
	typeMeta := metav1.TypeMeta{
		Kind:       gvk.Kind,
		APIVersion: gvk.GroupVersion().String(),
	}

	// Set value to field.
	field := value.FieldByName("TypeMeta")
	field.Set(reflect.ValueOf(typeMeta))
}

//nolint:tparallel
func TestVerifyOwnership(t *testing.T) {
	pkg := &corev1alpha1.Package{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pkg",
			Namespace: "default",
		},
		Spec: corev1alpha1.PackageSpec{
			Image: SuccessTestPackageImage,
			Config: &runtime.RawExtension{
				Raw: []byte(fmt.Sprintf(`{"testStubImage": "%s"}`, TestStubImage)),
			},
		},
	}

	// deploy package
	objectDeployment := &adapters.ObjectDeployment{}
	ctx := logr.NewContext(context.Background(), testr.New(t))
	requireDeployPackage(ctx, t, pkg, objectDeployment.ClientObject())

	// objectDeployment should reference its objectSet in '.status.controllerOf'
	controllerOf := objectDeployment.GetStatusControllerOf()
	require.Len(t, controllerOf, 1)
	objectSetReference := controllerOf[0]

	objectSet := &adapters.ObjectSet{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name:      objectSetReference.Name,
		Namespace: objectSetReference.Namespace,
	}, objectSet.ClientObject()))

	// objectSet should reference the deployment in '.status.controllerOf'
	controllerOf = objectSet.GetStatusControllerOf()
	require.Len(t, controllerOf, 1)
	deploymentReference := controllerOf[0]

	deployment := &appsv1.Deployment{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name:      deploymentReference.Name,
		Namespace: deploymentReference.Namespace,
	}, deployment))

	ownershipChain := []client.Object{
		pkg,
		objectDeployment.ClientObject(),
		objectSet.ClientObject(),
		deployment,
	}

	for _, obj := range ownershipChain {
		requireSetTypeMeta(t, obj, Scheme)
	}

	t.Run("all possible links", func(t *testing.T) {
		t.Parallel()

		for i := range ownershipChain {
			for j := range ownershipChain {
				parent := ownershipChain[i]
				child := ownershipChain[j]

				msg := fmt.Sprintf("parent: %s, child: %s",
					parent.GetObjectKind().GroupVersionKind().String(),
					child.GetObjectKind().GroupVersionKind().String())

				isOwner, err := ownership.VerifyOwnership(child, parent)

				switch i {
				case j - 1:
					// link should be valid
					require.NoError(t, err, msg)
					assert.True(t, isOwner, msg)
				case 3:
					// parent is 'deployment' --> invalid owner kind
					require.ErrorIs(t, err, ownership.ErrUnsupportedOwnerKind, msg)
				default:
					// link should be invalid
					require.NoError(t, err, msg)
					assert.False(t, isOwner, msg)
				}
			}
		}
	})

	t.Run("owner reference tampering", func(t *testing.T) {
		t.Parallel()

		tr := true
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "default",
				UID:       "test-cm-uid",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "package-operator.run/v1alpha1",
						Kind:               "ObjectSet",
						Name:               objectSet.GetName(),
						UID:                objectSet.GetUID(),
						Controller:         &tr,
						BlockOwnerDeletion: nil,
					},
				},
			},
		}

		require.NoError(t, Client.Create(ctx, cm))
		cleanupOnSuccess(ctx, t, cm)

		require.NoError(t, Client.Get(ctx, client.ObjectKey{
			Name:      "test-cm",
			Namespace: "default",
		}, cm))

		// make sure cm doesn't belong to any PKO objects in the ownership chain
		// (deployment would error with unsupported kind, so it's skipped)
		for _, owner := range ownershipChain[:3] {
			isOwner, err := ownership.VerifyOwnership(cm, owner)
			require.NoError(t, err)
			assert.False(t, isOwner)
		}
	})
}
