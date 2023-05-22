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
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func TestUnpackReconciler(t *testing.T) {
	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(ipm, pd, nil)

	const image = "test123:latest"

	f := packagecontent.Files{}
	ipm.
		On("Pull", mock.Anything, mock.Anything).
		Return(f, nil)
	pd.
		On("Load", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

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
	assert.True(t, res.IsZero())

	assert.True(t,
		meta.IsStatusConditionTrue(*pkg.GetConditions(),
			corev1alpha1.PackageUnpacked))
	assert.NotEmpty(t, pkg.GetSpecHash())
}

func TestUnpackReconciler_noop(t *testing.T) {
	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(ipm, pd, nil)

	const image = "test123:latest"

	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			Spec: corev1alpha1.PackageSpec{
				Image: image,
			},
		},
	}
	pkg.Package.Status.UnpackedHash = pkg.GetSpecHash()
	ctx := context.Background()
	res, err := ur.Reconcile(ctx, pkg)
	require.NoError(t, err)
	assert.True(t, res.IsZero())
}

var errTest = errors.New("test error")

func TestUnpackReconciler_pullBackoff(t *testing.T) {
	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(ipm, pd, nil)

	const image = "test123:latest"

	f := packagecontent.Files{}
	ipm.
		On("Pull", mock.Anything, mock.Anything).
		Return(f, errTest)

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
) (packagecontent.Files, error) {
	args := m.Called(ctx, image)
	return args.Get(0).(packagecontent.Files), args.Error(1)
}

type packageDeployerMock struct {
	mock.Mock
}

func (m *packageDeployerMock) Load(
	ctx context.Context, pkg adapters.GenericPackageAccessor,
	files packagecontent.Files, env manifestsv1alpha1.PackageEnvironment,
) error {
	args := m.Called(ctx, pkg, files, env)
	return args.Error(0)
}
