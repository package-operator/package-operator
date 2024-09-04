//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/autoimpersonation/ownership"
	"package-operator.run/internal/controllers/objectsets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestVerifyOwnership(t *testing.T) {
	pkg := &corev1alpha1.Package{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Package",
			APIVersion: "package-operator.run/v1alpha1",
		},
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
	objectDeployment := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectDeployment",
				APIVersion: "package-operator.run/v1alpha1",
			},
		},
	}
	ctx := logr.NewContext(context.Background(), testr.New(t))
	requireDeployPackage(ctx, t, pkg, objectDeployment.ClientObject())

	// objectDeployment should reference its objectSet in '.status.controllerOf'
	controllerOf := objectDeployment.GetStatusControllerOf()
	require.Len(t, controllerOf, 1)
	objectSetReference := controllerOf[0]

	objectSet := &objectsets.GenericObjectSet{
		ObjectSet: corev1alpha1.ObjectSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ObjectSet",
				APIVersion: "package-operator.run/v1alpha1",
			},
		},
	}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name:      objectSetReference.Name,
		Namespace: objectSetReference.Namespace,
	}, &objectSet.ObjectSet))

	// objectSet should reference the deployment in '.status.controllerOf'
	controllerOf = objectSet.GetStatusControllerOf()
	require.Len(t, controllerOf, 1)
	deploymentReference := controllerOf[0]

	deployment := &appsv1.Deployment{}
	require.NoError(t, Client.Get(ctx, client.ObjectKey{
		Name:      deploymentReference.Name,
		Namespace: deploymentReference.Namespace,
	}, deployment))

	objectSet.ObjectSet.TypeMeta = metav1.TypeMeta{
		Kind:       "ObjectSet",
		APIVersion: "package-operator.run/v1alpha1",
	}

	ownershipChain := []client.Object{
		pkg,
		objectDeployment,
		objectSet,
		deployment,
	}

	fmt.Printf("%#v\n", pkg.GetObjectKind())
	fmt.Printf("%#v\n", objectDeployment.GetObjectKind())
	fmt.Printf("%#v\n", objectSet.GetObjectKind())
	fmt.Printf("%#v\n", deployment.GetObjectKind())

	t.Run("all possible links", func(t *testing.T) {
		t.Parallel()

		for i := 0; i < len(ownershipChain); i++ {
			for j := 0; j < len(ownershipChain); j++ {
				parent := ownershipChain[i]
				child := ownershipChain[j]
				isOwner, err := ownership.VerifyOwnership(child, parent)

				if i+1 == j {
					// link should be valid
					require.NoError(t, err)
					assert.True(t, isOwner)
				} else if i == 3 {
					// parent is 'deployment' --> invalid owner kind
					assert.ErrorIs(t, err, ownership.ErrUnsupportedOwnerKind)
				} else {
					// link should be invalid
					require.NoError(t, err)
					assert.False(t, isOwner)
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
