package teardown_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apps "k8s.io/api/apps/v1"
	rbac "k8s.io/api/rbac/v1"
	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkoapis "package-operator.run/apis"
	pkocore "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/cmd/package-operator-manager/components"
	"package-operator.run/cmd/package-operator-manager/teardown"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := apps.AddToScheme(testScheme); err != nil {
		panic(err)
	}
	if err := ext.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

// TestTeardown checks that the teardown manager actually deletes all the things.
func TestTeardown(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))
	somepkg := pkocore.Package{ObjectMeta: meta.ObjectMeta{Namespace: "testnamespace", Name: "testpkg"}}

	// It get all packages first.
	c.On("List", mock.Anything, mock.IsType(&pkocore.PackageList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*pkocore.PackageList)
			require.Len(t, args.Get(2).([]client.ListOption), 1)
			list.Items = append(list.Items, somepkg)
		},
	)

	// Deletes each package.
	c.On("Delete", mock.Anything, mock.IsType(&pkocore.Package{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			pkg := args.Get(1).(*pkocore.Package)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, somepkg, *pkg)
		},
	)

	// Checks that the package is gone.
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocore.Package{}), mock.IsType([]client.GetOption{})).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "")).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Namespace: somepkg.Namespace, Name: somepkg.Name}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
		},
	)

	// Delete cluster packages.

	someClusterPkg := pkocore.ClusterPackage{ObjectMeta: meta.ObjectMeta{Name: "testpkg"}}
	pkoClusterPkg := pkocore.ClusterPackage{ObjectMeta: meta.ObjectMeta{Name: "package-operator", Finalizers: []string{"package-operator.run/teardown-job"}}}

	// Gets all cluster packages.
	c.On("List", mock.Anything, mock.IsType(&pkocore.ClusterPackageList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*pkocore.ClusterPackageList)
			require.Len(t, args.Get(2).([]client.ListOption), 0)
			list.Items = append(list.Items, someClusterPkg, pkoClusterPkg)
		},
	)

	// Deletes each cluster package except the PKO one.
	c.On("Delete", mock.Anything, mock.IsType(&pkocore.ClusterPackage{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			pkg := args.Get(1).(*pkocore.ClusterPackage)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, someClusterPkg, *pkg)
		},
	)

	// Checks that the cluster package is gone.
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocore.ClusterPackage{}), mock.IsType([]client.GetOption{})).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "")).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Name: somepkg.Name}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
		},
	)

	// Get PKO cluster package.
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocore.ClusterPackage{}), mock.IsType([]client.GetOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Name: "package-operator"}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
			a := args.Get(2).(*pkocore.ClusterPackage)
			*a = pkoClusterPkg
		},
	)

	// Remove teardown finalizer out of PKO clusterpackage.
	c.On("Patch", mock.Anything, mock.IsType(&pkocore.ClusterPackage{}), mock.Anything, mock.IsType([]client.PatchOption{})).Once().Return(nil)

	cobsl := pkocore.ClusterObjectSetList{
		Items: []pkocore.ClusterObjectSet{
			{ObjectMeta: meta.ObjectMeta{Name: "cheese", Finalizers: []string{"package-operator.run/cached"}}},
			{ObjectMeta: meta.ObjectMeta{Name: "burger", Finalizers: []string{}}},
		},
	}

	// List all cluster object sets
	c.On("List", mock.Anything, mock.IsType(&pkocore.ClusterObjectSetList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*pkocore.ClusterObjectSetList)
			require.Len(t, args.Get(2).([]client.ListOption), 0)
			list.Items = append(list.Items, cobsl.Items...)
		},
	)

	// Remove cache finalizer from each cluster object set.
	// This is done because PKO does not clean it up.
	c.On("Patch", mock.Anything, mock.IsType(&pkocore.ClusterObjectSet{}), mock.Anything, mock.IsType([]client.PatchOption{})).Once().Return(nil)

	// List all CRDs that are owned by the PKO package.
	crdl := ext.CustomResourceDefinitionList{Items: []ext.CustomResourceDefinition{{ObjectMeta: meta.ObjectMeta{Name: "burger", Finalizers: []string{}}}}}
	c.On("List", mock.Anything, mock.IsType(&ext.CustomResourceDefinitionList{}), mock.IsType([]client.ListOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			list := args.Get(1).(*ext.CustomResourceDefinitionList)
			require.Len(t, args.Get(2).([]client.ListOption), 1)
			list.Items = append(list.Items, crdl.Items...)
		},
	)
	// Delete each CRD.
	c.On("Delete", mock.Anything, mock.IsType(&ext.CustomResourceDefinition{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			pkg := args.Get(1).(*ext.CustomResourceDefinition)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, crdl.Items[0], *pkg)
		},
	)

	// Wait for the deleted CRD to be gone.
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&ext.CustomResourceDefinition{}), mock.IsType([]client.GetOption{})).Once().
		Return(k8serrors.NewNotFound(schema.GroupResource{}, "")).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Name: crdl.Items[0].Name}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
		},
	)

	// Delete cluster role.
	c.On("Delete", mock.Anything, mock.IsType(&rbac.ClusterRole{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			role := args.Get(1).(*rbac.ClusterRole)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, "package-operator-remote-phase-manager", role.Name)
		},
	)

	// Delete cluster role binding.
	c.On("Delete", mock.Anything, mock.IsType(&rbac.ClusterRoleBinding{}), mock.IsType([]client.DeleteOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			role := args.Get(1).(*rbac.ClusterRoleBinding)
			require.Len(t, args.Get(2).([]client.DeleteOption), 0)
			require.Equal(t, "package-operator", role.Name)
		},
	)

	require.NoError(t, teardown.NewTeardown(components.UncachedClient{Client: c}).Teardown(ctx))
}
