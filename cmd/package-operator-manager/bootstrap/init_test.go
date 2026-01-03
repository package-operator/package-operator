package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/testutil"
)

type (
	subTestFunc func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer)
	subTest     struct {
		name string
		t    subTestFunc
	}
)

func Test_initializer_ensureUpdatedPKO(t *testing.T) {
	t.Parallel()

	subTests := []subTest{
		{
			name: "PKOPackageNotExistent_NeedsBootstrap",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Once().Return(
					apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				c.On("Create",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Once().Return(nil)

				needsBootstrap, err := i.ensureUpdatedPKO(ctx)
				require.True(t, needsBootstrap)
				require.NoError(t, err)
				c.AssertExpectations(t)
			},
		},
		{
			name: "PKOPackageNotExistent_GetErrors",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Once().Return(
					apimachineryerrors.NewInternalError(errors.New("i'm just chillin' here")))

				needsBootstrap, err := i.ensureUpdatedPKO(ctx)
				require.False(t, needsBootstrap)
				require.True(t, apimachineryerrors.IsInternalError(err))
				c.AssertExpectations(t)
			},
		},
		{
			name: "PKOPackageNotExistent_CreateErrors",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Once().Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				c.On("Create",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Once().Return(
					apimachineryerrors.NewInternalError(errors.New("i'm just chillin' here")))

				_, err := i.ensureUpdatedPKO(ctx)
				require.True(t, apimachineryerrors.IsInternalError(err))
				c.AssertExpectations(t)
			},
		},
		{
			name: "PKOPackageExistentAndEqual_PKOUnavailable",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					pkg := args.Get(2).(*corev1alpha1.ClusterPackage)
					clPkg, err := i.newPKOClusterPackage()
					require.NoError(t, err)
					*pkg = *clPkg
				}).Return(nil)

				// it has to check if PKO deployment is available
				// mock an unavailable PKO and validate its deletion
				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					depl := args.Get(2).(*appsv1.Deployment)
					depl.Status.AvailableReplicas = 0
				}).Return(nil)

				// Bootstrap is needed to get PKO back up running
				needsBootstrap, err := i.ensureUpdatedPKO(ctx)
				require.True(t, needsBootstrap)
				require.NoError(t, err)
				c.AssertExpectations(t)
			},
		},
		{
			name: "PKOPackageExistentAndEqual_PKOAvailable",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					pkg := args.Get(2).(*corev1alpha1.ClusterPackage)
					clPkg, err := i.newPKOClusterPackage()
					require.NoError(t, err)
					*pkg = *clPkg
					pkg.Generation = 5
					meta.SetStatusCondition(&pkg.Status.Conditions, metav1.Condition{
						Type:               corev1alpha1.PackageAvailable,
						Status:             metav1.ConditionTrue,
						ObservedGeneration: pkg.Generation,
					})
				}).Return(nil)

				// it has to check if PKO deployment is available
				// mock an available PKO and validate that nothing else happens
				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					depl := args.Get(2).(*appsv1.Deployment)
					depl.Status.AvailableReplicas = 1
					depl.Status.UpdatedReplicas = depl.Status.AvailableReplicas
				}).Return(nil)

				needsBootstrap, err := i.ensureUpdatedPKO(ctx)
				require.False(t, needsBootstrap)
				require.NoError(t, err)
				c.AssertExpectations(t)
			},
		},

		{
			name: "PKOPackageExistentAndNotEqual_PKOUnavailable",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				// mock existing ClusterPackage with different spec
				existingPkg, err := i.newPKOClusterPackage()
				require.NoError(t, err)
				existingPkg.Spec.Image = "thisimagedoesnotexist.com"

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					pkg := args.Get(2).(*corev1alpha1.ClusterPackage)
					*pkg = *existingPkg
				}).Return(nil)

				// it has to check if PKO deployment is available
				// mock an available PKO and validate that nothing else happens
				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					depl := args.Get(2).(*appsv1.Deployment)
					depl.Status.AvailableReplicas = 0
				}).Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
					list.Items = append(list.Items, corev1alpha1.ClusterObjectSet{
						Spec: corev1alpha1.ClusterObjectSetSpec{
							LifecycleState: corev1alpha1.ObjectSetLifecycleStateActive,
						},
					})
				}).Return(nil)

				c.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.Anything,
				).Run(func(args mock.Arguments) {
					cos := args.Get(1).(*corev1alpha1.ClusterObjectSet)
					assert.Equal(t,
						corev1alpha1.ObjectSetLifecycleStatePaused,
						cos.Spec.LifecycleState)
				}).Return(nil)

				// mock already deleted PKO
				c.On("Delete",
					mock.Anything,
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				c.On("Patch",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterPackage{}),
					mock.Anything,
					mock.Anything,
				).Run(func(args mock.Arguments) {
					pkg := args.Get(1).(*corev1alpha1.ClusterPackage)
					// validate that pkg is NOT equal to the existing pkg in the cluster
					require.False(t, equality.Semantic.DeepEqual(pkg, existingPkg))
				}).Return(nil)

				needsBootstrap, err := i.ensureUpdatedPKO(ctx)
				require.True(t, needsBootstrap)
				require.NoError(t, err)
				c.AssertExpectations(t)
			},
		},
	}

	for _, subTest := range subTests {
		c := testutil.NewClient()
		ctx := logr.NewContext(context.Background(), testr.New(t))
		t.Run(subTest.name, func(t *testing.T) {
			t.Parallel()
			subTest.t(
				t,
				c,
				ctx,
				&initializer{
					client: c,
				})
		})
	}
}

func Test_initializer_ensurePKORevisionsPaused(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()
	init := &initializer{
		client: c,
	}

	// Should be paused
	cos1 := &corev1alpha1.ClusterObjectSet{}
	cos1.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateActive

	// Should be unpaused
	cos2 := &corev1alpha1.ClusterObjectSet{}
	cos2.Annotations = map[string]string{
		manifestsv1alpha1.PackageSourceImageAnnotation: "quay.io/xxx",
	}
	cos2.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStatePaused
	cosList := &corev1alpha1.ClusterObjectSetList{}
	cosList.Items = []corev1alpha1.ClusterObjectSet{
		*cos1, *cos2,
	}

	c.On("List",
		mock.Anything,
		mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		l := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
		*l = *cosList
	}).Return(nil)

	var updatedCOS []corev1alpha1.ClusterObjectSet
	c.On("Update",
		mock.Anything,
		mock.IsType(&corev1alpha1.ClusterObjectSet{}),
		mock.Anything,
	).Run(func(args mock.Arguments) {
		cos := args.Get(1).(*corev1alpha1.ClusterObjectSet)
		updatedCOS = append(updatedCOS, *cos)
	}).Return(nil)

	ctx := context.Background()
	err := init.ensurePKORevisionsPaused(ctx, &corev1alpha1.ClusterPackage{
		Spec: corev1alpha1.PackageSpec{
			Image: "quay.io/xxx",
		},
	})
	require.NoError(t, err)

	if assert.Len(t, updatedCOS, 2) {
		assert.Equal(t,
			corev1alpha1.ObjectSetLifecycleStatePaused, updatedCOS[0].Spec.LifecycleState)
		assert.Equal(t,
			corev1alpha1.ObjectSetLifecycleStateActive, updatedCOS[1].Spec.LifecycleState)
	}
}

func Test_initializer_ensureDeploymentGone(t *testing.T) {
	t.Parallel()

	subTests := []subTest{
		{
			name: "DeploymentNotFound",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				c.On("Delete",
					mock.Anything,
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				err := i.ensurePKODeploymentGone(ctx)
				require.NoError(t, err)
				c.AssertExpectations(t)
			},
		},
		{
			name: "DeploymentFound",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, i *initializer) {
				t.Helper()

				c.On("Delete",
					mock.Anything,
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Return(nil)

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&appsv1.Deployment{}),
					mock.Anything,
				).Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				err := i.ensurePKODeploymentGone(ctx)
				require.NoError(t, err)
				c.AssertExpectations(t)
			},
		},
	}

	for _, subTest := range subTests {
		c := testutil.NewClient()
		ctx := logr.NewContext(context.Background(), testr.New(t))
		t.Run(subTest.name, func(t *testing.T) {
			t.Parallel()
			subTest.t(
				t,
				c,
				ctx,
				&initializer{
					client: c,
					scheme: testScheme,
				})
		})
	}
}

func Test_initializer_ensureCRDs(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &initializer{client: c}

	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))

	c.On("Create", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Return(apimachineryerrors.NewAlreadyExists(schema.GroupResource{}, ""))
	c.On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	c.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	crds := []unstructured.Unstructured{crd, crd}
	err := b.ensureCRDs(ctx, crds)
	require.NoError(t, err)

	for _, crd := range crds {
		assert.Equal(t, map[string]string{
			constants.DynamicCacheLabel: "True",
		}, crd.GetLabels())
	}
	c.AssertExpectations(t)
}

func Test_crdsFromObjects(t *testing.T) {
	t.Parallel()
	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))

	objs := []unstructured.Unstructured{
		{}, crd,
	}
	crds := crdsFromObjects(objs)
	assert.Len(t, crds, 1)
}
