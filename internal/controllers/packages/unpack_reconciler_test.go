package packages

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageimport"
	"package-operator.run/internal/testutil"
)

func TestUnpackReconciler(t *testing.T) {
	t.Parallel()

	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(nil, ipm, pd, nil, nil)

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
	ur.SetEnvironment(&manifestsv1alpha1.PackageEnvironment{
		Kubernetes: manifestsv1alpha1.PackageEnvironmentKubernetes{
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

	ipm.AssertExpectations(t)
	pd.AssertExpectations(t)
}

func TestUnpackReconciler_pullSecret(t *testing.T) {
	t.Parallel()

	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	c := testutil.NewClient()
	ur := newUnpackReconciler(c, ipm, pd, nil, nil)

	const image = "test123:latest"

	f := packagecontent.Files{}

	c.On("Get", mock.Anything, mock.Anything, mock.IsType(&corev1.Secret{}), mock.Anything).
		Run(func(args mock.Arguments) {
			nsn := args.Get(1).(types.NamespacedName)
			assert.Equal(t, "my-secret-object", nsn.Name)
			assert.Equal(t, "default", nsn.Namespace)
			*args.Get(2).(*corev1.Secret) = corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      nsn.Name,
					Namespace: nsn.Namespace,
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte("not-a-valid-docker-config-json"),
				},
			}
		}).
		Return(nil)
	ipm.
		On("Pull", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			opt := args.Get(2).(packageimport.PullOption)
			require.NotEmpty(t, opt)
			require.IsType(t, packageimport.WithPullSecret{}, opt)
			require.Equal(t, []byte("not-a-valid-docker-config-json"), opt.(packageimport.WithPullSecret).Data)
		}).
		Return(f, nil)
	pd.
		On("Load", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "default",
			},
			Spec: corev1alpha1.PackageSpec{
				Image:           image,
				ImagePullSecret: ptr.To[string]("pull-secret"),
				Secrets: []corev1alpha1.PackageSpecSecret{
					{
						Name: "pull-secret",
						SecretReference: corev1alpha1.SecretReference{
							Name:      "my-secret-object",
							Namespace: "trying-to-hack-another-namespace",
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	ur.SetEnvironment(&manifestsv1alpha1.PackageEnvironment{
		Kubernetes: manifestsv1alpha1.PackageEnvironmentKubernetes{
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

	c.AssertExpectations(t)
	ipm.AssertExpectations(t)
	pd.AssertExpectations(t)
}

func TestUnpackReconciler_noop(t *testing.T) {
	t.Parallel()

	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(nil, ipm, pd, nil, nil)

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

	ipm.AssertExpectations(t)
	pd.AssertExpectations(t)
}

var errTest = errors.New("test error")

func TestUnpackReconciler_pullBackoff(t *testing.T) {
	t.Parallel()

	ipm := &imagePullerMock{}
	pd := &packageDeployerMock{}
	ur := newUnpackReconciler(nil, ipm, pd, nil, nil)

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
	ctx context.Context, ref string, opts ...packageimport.PullOption,
) (packagecontent.Files, error) {
	actualArgs := []any{ctx, ref}
	for _, opt := range opts {
		actualArgs = append(actualArgs, opt)
	}

	args := m.Called(actualArgs...)
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
