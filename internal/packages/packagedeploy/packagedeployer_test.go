package packagedeploy

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/packages/packagestructure"
	"package-operator.run/package-operator/internal/testutil"
)

func TestNewPackageDeployer(t *testing.T) {
	c := testutil.NewClient()
	l := NewPackageDeployer(c, testScheme)
	assert.NotNil(t, l)
}

func TestNewClustePackageDeployer(t *testing.T) {
	c := testutil.NewClient()
	l := NewClusterPackageDeployer(c, testScheme)
	assert.NotNil(t, l)
}

func TestPackageDeployer_Load(t *testing.T) {
	c := testutil.NewClient()
	pcl := &packageContentLoaderMock{}
	deploymentReconcilerMock := &deploymentReconcilerMock{}
	l := &PackageDeployer{
		client:              c,
		scheme:              testScheme,
		newPackage:          newGenericPackage,
		newObjectDeployment: adapters.NewObjectDeployment,

		packageContentLoader: pcl,
		deploymentReconciler: deploymentReconcilerMock,
	}
	ctx := logr.NewContext(context.Background(), testr.New(t))

	c.On("Get",
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Return(nil)

	obj1 := unstructured.Unstructured{Object: map[string]interface{}{}}
	obj1.SetAnnotations(map[string]string{
		manifestsv1alpha1.PackagePhaseAnnotation: "phase-1",
	})
	res := &packagestructure.PackageContent{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			Spec: manifestsv1alpha1.PackageManifestSpec{
				Scopes: []manifestsv1alpha1.PackageManifestScope{
					manifestsv1alpha1.PackageManifestScopeNamespaced,
				},
				Phases: []manifestsv1alpha1.PackageManifestPhase{
					{
						Name: "phase-1",
					},
					{
						Name: "empty-phase",
					},
				},
			},
		},
		Manifests: packagestructure.ManifestMap{
			"file1.yaml": []unstructured.Unstructured{obj1},
		},
	}
	pcl.On("LoadFromPath",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(res, nil)

	deploymentReconcilerMock.
		On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

		// retries on conflict
	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Once().
		Return(apierrors.NewConflict(schema.GroupResource{}, "", nil))

	var updatedPackage *corev1alpha1.Package
	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			updatedPackage = args.Get(1).(*corev1alpha1.Package)
		}).
		Return(nil)

	pkgKey := client.ObjectKey{
		Name: "test", Namespace: "test",
	}
	folderPath := "folder-path"
	err := l.Load(ctx, pkgKey, folderPath)
	require.NoError(t, err)

	packageInvalid := meta.FindStatusCondition(updatedPackage.Status.Conditions, corev1alpha1.PackageInvalid)
	if assert.NotNil(t, packageInvalid) {
		assert.Equal(t, metav1.ConditionFalse, packageInvalid.Status)
		assert.Equal(t, packageInvalid.Reason, "LoadSuccess")
		assert.Equal(t, packageInvalid.Message, "")
	}
}

var errExample = errors.New("example error")

func TestPackageDeployer_Load_Error(t *testing.T) {
	c := testutil.NewClient()
	pcl := &packageContentLoaderMock{}
	deploymentReconcilerMock := &deploymentReconcilerMock{}
	l := &PackageDeployer{
		client:              c,
		scheme:              testScheme,
		newPackage:          newGenericPackage,
		newObjectDeployment: adapters.NewObjectDeployment,

		packageContentLoader: pcl,
		deploymentReconciler: deploymentReconcilerMock,
	}
	ctx := logr.NewContext(context.Background(), testr.New(t))

	c.On("Get",
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Return(nil)

	obj1 := unstructured.Unstructured{Object: map[string]interface{}{}}
	obj1.SetAnnotations(map[string]string{
		manifestsv1alpha1.PackagePhaseAnnotation: "phase-1",
	})
	var res *packagestructure.PackageContent
	pcl.On("LoadFromPath",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(res, errExample)

	deploymentReconcilerMock.
		On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

		// retries on conflict
	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Once().
		Return(apierrors.NewConflict(schema.GroupResource{}, "", nil))

	var updatedPackage *corev1alpha1.Package
	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			updatedPackage = args.Get(1).(*corev1alpha1.Package)
		}).
		Return(nil)

	pkgKey := client.ObjectKey{
		Name: "test", Namespace: "test",
	}
	folderPath := "folder-path"
	err := l.Load(ctx, pkgKey, folderPath)
	require.NoError(t, err)

	packageInvalid := meta.FindStatusCondition(updatedPackage.Status.Conditions, corev1alpha1.PackageInvalid)
	if assert.NotNil(t, packageInvalid) {
		assert.Equal(t, metav1.ConditionTrue, packageInvalid.Status)
		assert.Equal(t, packageInvalid.Reason, "LoadError")
		assert.Equal(t, packageInvalid.Message, "example error")
	}
}

var _ deploymentReconciler = (*deploymentReconcilerMock)(nil)

type deploymentReconcilerMock struct {
	mock.Mock
}

func (m *deploymentReconcilerMock) Reconcile(
	ctx context.Context, desiredDeploy adapters.ObjectDeploymentAccessor,
	chunker objectChunker,
) error {
	args := m.Called(ctx, desiredDeploy, chunker)
	return args.Error(0)
}

var _ packageContentLoader = (*packageContentLoaderMock)(nil)

type packageContentLoaderMock struct {
	mock.Mock
}

func (m *packageContentLoaderMock) LoadFromPath(ctx context.Context, path string, opts ...packagestructure.LoaderOption) (
	*packagestructure.PackageContent, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(*packagestructure.PackageContent), args.Error(1)
}
