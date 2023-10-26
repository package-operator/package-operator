package teardown_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkoapis "package-operator.run/apis"
	pkocorev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/cmd/package-operator-manager/components"
	"package-operator.run/cmd/package-operator-manager/teardown"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := appsv1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := extv1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

// TestTeardown checks that the teardown manager actually deletes all the things.
func TestTeardown(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))
	somepkg := pkocorev1alpha1.Package{ObjectMeta: metav1.ObjectMeta{Namespace: "testnamespace", Name: "testpkg"}}

	// List all CRDs that are owned by the PKO package.
	crdl := extv1.CustomResourceDefinitionList{Items: []extv1.CustomResourceDefinition{{ObjectMeta: metav1.ObjectMeta{Name: "burger", Finalizers: []string{}}}}}
	c.On("List", mock.Anything, mock.IsType(&extv1.CustomResourceDefinitionList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*extv1.CustomResourceDefinitionList)
			require.Len(t, args.Get(2).([]client.ListOption), 1)
			list.Items = append(list.Items, crdl.Items...)
		},
	)

	// Delete each CRD.
	c.On("Delete", mock.Anything, mock.IsType(&extv1.CustomResourceDefinition{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			pkg := args.Get(1).(*extv1.CustomResourceDefinition)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, crdl.Items[0], *pkg)
		},
	)

	// Wait for the deleted CRD to be gone.
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&extv1.CustomResourceDefinition{}), mock.IsType([]client.GetOption{})).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "")).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Name: crdl.Items[0].Name}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
		},
	)

	// Remove teardown finalizer out of PKO clusterpackage.
	c.On("Patch", mock.Anything, mock.IsType(&pkocorev1alpha1.ClusterPackage{}), mock.Anything, mock.IsType([]client.PatchOption{})).Once().Return(nil)

	cobsl := pkocorev1alpha1.ClusterObjectSetList{
		Items: []pkocorev1alpha1.ClusterObjectSet{
			{ObjectMeta: metav1.ObjectMeta{Name: "cheese", Finalizers: []string{"package-operator.run/cached"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "burger", Finalizers: []string{}}},
		},
	}

	// List all cluster object sets
	c.On("List", mock.Anything, mock.IsType(&pkocorev1alpha1.ClusterObjectSetList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*pkocorev1alpha1.ClusterObjectSetList)
			require.Len(t, args.Get(2).([]client.ListOption), 0)
			list.Items = append(list.Items, cobsl.Items...)
		},
	)

	// Remove cache finalizer from each cluster object set.
	// This is done because PKO does not clean it up.
	c.On("Patch", mock.Anything, mock.IsType(&pkocorev1alpha1.ClusterObjectSet{}), mock.Anything, mock.IsType([]client.PatchOption{})).Once().Return(nil)

	// It gets all packages first.
	c.On("List", mock.Anything, mock.IsType(&pkocorev1alpha1.PackageList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*pkocorev1alpha1.PackageList)
			require.Len(t, args.Get(2).([]client.ListOption), 1)
			list.Items = append(list.Items, somepkg)
		},
	)

	// Deletes each package.
	c.On("Delete", mock.Anything, mock.IsType(&pkocorev1alpha1.Package{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			pkg := args.Get(1).(*pkocorev1alpha1.Package)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, somepkg, *pkg)
		},
	)

	// Checks that the package is gone.
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocorev1alpha1.Package{}), mock.IsType([]client.GetOption{})).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "")).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Namespace: somepkg.Namespace, Name: somepkg.Name}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
		},
	)

	// Delete cluster packages.

	someClusterPkg := pkocorev1alpha1.ClusterPackage{ObjectMeta: metav1.ObjectMeta{Name: "testpkg"}}
	pkoClusterPkg := pkocorev1alpha1.ClusterPackage{ObjectMeta: metav1.ObjectMeta{Name: "package-operator", Finalizers: []string{"package-operator.run/teardown-job"}}}

	// Gets all cluster packages.
	c.On("List", mock.Anything, mock.IsType(&pkocorev1alpha1.ClusterPackageList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*pkocorev1alpha1.ClusterPackageList)
			require.Len(t, args.Get(2).([]client.ListOption), 0)
			list.Items = append(list.Items, someClusterPkg, pkoClusterPkg)
		},
	)

	// Deletes each cluster package except the PKO one.
	c.On("Delete", mock.Anything, mock.IsType(&pkocorev1alpha1.ClusterPackage{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			pkg := args.Get(1).(*pkocorev1alpha1.ClusterPackage)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, someClusterPkg, *pkg)
		},
	)

	// Checks that the cluster package is gone.
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocorev1alpha1.ClusterPackage{}), mock.IsType([]client.GetOption{})).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "")).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Name: someClusterPkg.Name}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
		},
	)

	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocorev1alpha1.ClusterPackage{}), mock.IsType([]client.GetOption{})).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "")).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Name: pkoClusterPkg.Name}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
			a := args.Get(2).(*pkocorev1alpha1.ClusterPackage)
			*a = pkoClusterPkg
		},
	)

	// Delete cluster role.
	c.On("Delete", mock.Anything, mock.IsType(&rbacv1.ClusterRole{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			role := args.Get(1).(*rbacv1.ClusterRole)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, "package-operator-remote-phase-manager", role.Name)
		},
	)

	// Delete cluster role binding.
	c.On("Delete", mock.Anything, mock.IsType(&rbacv1.ClusterRoleBinding{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			role := args.Get(1).(*rbacv1.ClusterRoleBinding)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, "package-operator", role.Name)
		},
	)

	require.NoError(t, teardown.NewTeardown(components.UncachedClient{Client: c}).Teardown(ctx))
}
