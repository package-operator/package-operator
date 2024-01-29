package packages

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/packages"
	"package-operator.run/internal/testutil"
)

func TestUnpackReconciler(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()

	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(c, ipm, pd, nil, nil)

	const image = "test123:latest"

	rawPkg := &packages.RawPackage{}
	ipm.
		On("Pull", mock.Anything, mock.Anything).
		Return(rawPkg, nil)
	pd.
		On("Deploy", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			Spec: corev1alpha1.PackageSpec{
				Image: image,
			},
		},
	}
	ctx := context.Background()
	ur.SetEnvironment(&manifests.PackageEnvironment{
		Kubernetes: manifests.PackageEnvironmentKubernetes{
			Version: "v11111",
		},
	})
	res, err := ur.Reconcile(ctx, pkg)
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	assert.True(t,
		meta.IsStatusConditionTrue(*pkg.GetConditions(),
			corev1alpha1.PackageUnpacked))
	assert.NotEmpty(t, pkg.GetSpecHash(nil))
}

func TestUnpackReconciler_noop(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()

	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(c, ipm, pd, nil, nil)

	const image = "test123:latest"

	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			Spec: corev1alpha1.PackageSpec{
				Image: image,
			},
		},
	}
	pkg.Package.Status.UnpackedHash = pkg.GetSpecHash(nil)
	ctx := context.Background()
	res, err := ur.Reconcile(ctx, pkg)
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

var errTest = errors.New("test error")

func TestUnpackReconciler_pullBackoff(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()

	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(c, ipm, pd, nil, nil)

	const image = "test123:latest"

	rawPkg := &packages.RawPackage{}
	ipm.
		On("Pull", mock.Anything, mock.Anything).
		Return(rawPkg, errTest)

	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			Spec: corev1alpha1.PackageSpec{
				Image: image,
			},
		},
	}

	ctx := context.Background()
	res, err := ur.Reconcile(ctx, pkg)
	require.NoError(t, err)
	assert.Equal(t, controllers.DefaultInitialBackoff, res.RequeueAfter)

	assert.True(t,
		meta.IsStatusConditionFalse(*pkg.GetConditions(),
			corev1alpha1.PackageUnpacked))
}

type imagePullerMock struct {
	mock.Mock
}

func (m *imagePullerMock) Pull(
	ctx context.Context, image string,
) (*packages.RawPackage, error) {
	args := m.Called(ctx, image)
	return args.Get(0).(*packages.RawPackage), args.Error(1)
}

type packageDeployerMock struct {
	mock.Mock
}

func (m *packageDeployerMock) Deploy(
	ctx context.Context,
	apiPkg adapters.GenericPackageAccessor,
	rawPkg *packages.RawPackage,
	env manifests.PackageEnvironment,
) error {
	args := m.Called(ctx, apiPkg, rawPkg, env)
	return args.Error(0)
}
