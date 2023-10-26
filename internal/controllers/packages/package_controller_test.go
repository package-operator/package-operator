package packages_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pkoapis "package-operator.run/apis"
	pkocore "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/packages"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := pkoapis.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestGenericPackageControllerHandleDeletion(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	log := testr.New(t)
	ctx := logr.NewContext(context.Background(), testr.New(t))

	name := types.NamespacedName{Name: "björk"}
	image := "bjork"
	namespace := "cheeseburgaz"
	serviceAccount := "wheel"

	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocore.ClusterPackage{}), mock.IsType([]client.GetOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			require.Equal(t, name, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
			*args.Get(2).(*pkocore.ClusterPackage) = pkocore.ClusterPackage{
				ObjectMeta: meta.ObjectMeta{Name: "package-operator", DeletionTimestamp: &meta.Time{Time: time.Now()}},
			}
		},
	)

	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&apps.Deployment{}), mock.IsType([]client.GetOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			require.Equal(t, types.NamespacedName{Namespace: "package-operator-system", Name: "package-operator-manager"}, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
			*args.Get(2).(*apps.Deployment) = apps.Deployment{
				ObjectMeta: meta.ObjectMeta{Namespace: namespace},
				Spec: apps.DeploymentSpec{
					Template: core.PodTemplateSpec{
						Spec: core.PodSpec{
							ServiceAccountName: serviceAccount,
							Containers: []core.Container{
								{Image: image},
							},
						},
					},
				},
			}
		},
	)

	c.On("Create", mock.Anything, mock.IsType(&batch.Job{}), mock.IsType([]client.CreateOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			job := *args.Get(1).(*batch.Job)
			require.Equal(t, "package-operator-teardown-job", job.Name)
			require.Equal(t, namespace, job.Namespace)
			require.Equal(t, serviceAccount, job.Spec.Template.Spec.ServiceAccountName)
			require.Equal(t, image, job.Spec.Template.Spec.Containers[0].Image)
			require.Len(t, args.Get(2).([]client.CreateOption), 0)
		},
	)

	controller := packages.NewClusterPackageController(c, log, testScheme, nil, nil, nil)
	res, err := controller.Reconcile(ctx, reconcile.Request{NamespacedName: name})
	require.NoError(t, err)
	require.True(t, res.IsZero())
}

func TestGenericPackageControllerHandleDeletionOtherPKG(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	log := testr.New(t)
	ctx := logr.NewContext(context.Background(), testr.New(t))

	name := types.NamespacedName{Name: "björk"}
	c.On("Get", mock.Anything, mock.IsType(types.NamespacedName{}), mock.IsType(&pkocore.ClusterPackage{}), mock.IsType([]client.GetOption{})).Once().
		Return(nil).Run(
		func(args mock.Arguments) {
			require.Equal(t, name, args.Get(1).(types.NamespacedName))
			require.Len(t, args.Get(3).([]client.GetOption), 0)
			*args.Get(2).(*pkocore.ClusterPackage) = pkocore.ClusterPackage{
				ObjectMeta: meta.ObjectMeta{Name: "not-package-operator", DeletionTimestamp: &meta.Time{Time: time.Now()}},
			}
		},
	)

	controller := packages.NewClusterPackageController(c, log, testScheme, nil, nil, nil)
	res, err := controller.Reconcile(ctx, reconcile.Request{NamespacedName: name})
	require.NoError(t, err)
	require.True(t, res.IsZero())
}
