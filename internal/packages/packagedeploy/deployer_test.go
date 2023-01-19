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
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageloader"
	"package-operator.run/package-operator/internal/testutil"
)

var (
	_          deploymentReconciler = (*deploymentReconcilerMock)(nil)
	_          packageContentLoader = (*packageContentLoaderMock)(nil)
	errExample                      = errors.New("example error")
)

type (
	deploymentReconcilerMock struct {
		mock.Mock
	}

	packageContentLoaderMock struct {
		mock.Mock
	}
)

func TestNewPackageDeployer(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	l := NewPackageDeployer(c, testScheme)
	assert.NotNil(t, l)
}

func TestNewClustePackageDeployer(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	l := NewClusterPackageDeployer(c, testScheme)
	assert.NotNil(t, l)
}

func TestPackageDeployer_Load(t *testing.T) {
	t.Parallel()

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
	res := &packagecontent.Package{
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
		Objects: map[string][]unstructured.Unstructured{"file1.yaml": {obj1}},
	}
	pcl.On("FromFiles", mock.Anything, mock.Anything, mock.Anything).Return(res, nil)

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
	files := packagecontent.Files{}
	err := l.Load(ctx, pkgKey, files)
	require.NoError(t, err)

	packageInvalid := meta.FindStatusCondition(updatedPackage.Status.Conditions, corev1alpha1.PackageInvalid)
	if assert.NotNil(t, packageInvalid) {
		assert.Equal(t, metav1.ConditionFalse, packageInvalid.Status)
		assert.Equal(t, packageInvalid.Reason, "LoadSuccess")
		assert.Equal(t, packageInvalid.Message, "")
	}
}

func TestPackageDeployer_Load_Error(t *testing.T) {
	t.Parallel()

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
	var res *packagecontent.Package
	pcl.On("FromFiles", mock.Anything, mock.Anything, mock.Anything).Return(res, errExample)

	deploymentReconcilerMock.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).Return(nil)

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
	files := packagecontent.Files{}
	err := l.Load(ctx, pkgKey, files)
	require.NoError(t, err)

	packageInvalid := meta.FindStatusCondition(updatedPackage.Status.Conditions, corev1alpha1.PackageInvalid)
	if assert.NotNil(t, packageInvalid) {
		assert.Equal(t, metav1.ConditionTrue, packageInvalid.Status)
		assert.Equal(t, packageInvalid.Reason, "LoadError")
		assert.Equal(t, packageInvalid.Message, "example error")
	}
}

func (m *deploymentReconcilerMock) Reconcile(
	ctx context.Context, desiredDeploy adapters.ObjectDeploymentAccessor,
	chunker objectChunker,
) error {
	args := m.Called(ctx, desiredDeploy, chunker)
	return args.Error(0)
}

func (m *packageContentLoaderMock) FromFiles(
	ctx context.Context, path packagecontent.Files, opts ...packageloader.Option) (*packagecontent.Package, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(*packagecontent.Package), args.Error(1)
}
