package packages

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/adapters"
	"package-operator.run/package-operator/internal/testutil"
)

func TestNewPackageLoader(t *testing.T) {
	c := testutil.NewClient()
	l := NewPackageLoader(c, testScheme)
	assert.NotNil(t, l)
}

func TestNewClustePackageLoader(t *testing.T) {
	c := testutil.NewClient()
	l := NewClusterPackageLoader(c, testScheme)
	assert.NotNil(t, l)
}

func TestPackageLoader_Load(t *testing.T) {
	c := testutil.NewClient()
	flm := &folderLoaderMock{}
	deploymentReconcilerMock := &deploymentReconcilerMock{}
	l := &PackageLoader{
		client:              c,
		scheme:              testScheme,
		newPackage:          newGenericPackage,
		newObjectDeployment: adapters.NewObjectDeployment,

		folderLoader:         flm,
		deploymentReconciler: deploymentReconcilerMock,
	}
	ctx := logr.NewContext(context.Background(), testr.New(t))

	c.On("Get",
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Return(nil)

	res := FolderLoaderResult{
		Manifest: &manifestsv1alpha1.PackageManifest{
			Spec: manifestsv1alpha1.PackageManifestSpec{
				Scopes: []manifestsv1alpha1.PackageManifestScope{
					manifestsv1alpha1.PackageManifestScopeNamespaced,
				},
			},
		},
	}
	flm.On("Load",
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
		Return(errors.NewConflict(schema.GroupResource{}, "", nil))

	c.StatusMock.On("Update",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Package"),
		mock.Anything).
		Return(nil)

	pkgKey := client.ObjectKey{
		Name: "test", Namespace: "test",
	}
	folderPath := "folder-path"
	err := l.Load(ctx, pkgKey, folderPath)
	require.NoError(t, err)
}

func TestPackageLoader_loadFromFolder_checksScope(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))
	flm := &folderLoaderMock{}
	l := &PackageLoader{
		folderLoader: flm,
	}

	res := FolderLoaderResult{
		Manifest: &manifestsv1alpha1.PackageManifest{},
	}
	flm.On("Load",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(res, nil)

	pkg := &GenericPackage{}

	_, err := l.loadFromFolder(ctx, pkg, "")
	assert.EqualError(t, err, "Package does not support Namespaced scope, supported scopes are: ")
}

func Test_setInvalidConditionBasedOnLoadError(t *testing.T) {
	tests := []struct {
		expectedConditionReason string
		pkg                     genericPackage
		err                     error
	}{
		{
			expectedConditionReason: "PackageManifestNotFound",
			pkg:                     &GenericPackage{},
			err:                     &PackageManifestNotFoundError{},
		},
		{
			expectedConditionReason: "PackageManifestInvalid",
			pkg:                     &GenericPackage{},
			err:                     &PackageManifestInvalidError{},
		},
		{
			expectedConditionReason: "InvalidScope",
			pkg:                     &GenericPackage{},
			err:                     &PackageInvalidScopeError{},
		},
		{
			expectedConditionReason: "InvalidObject",
			pkg:                     &GenericPackage{},
			err:                     &PackageObjectInvalidError{},
		},
	}

	for _, test := range tests {
		t.Run(test.expectedConditionReason, func(t *testing.T) {
			setInvalidConditionBasedOnLoadError(test.pkg, test.err)
			conds := *test.pkg.GetConditions()
			if assert.Len(t, conds, 1) {
				assert.Equal(t, test.expectedConditionReason, conds[0].Reason)
			}
		})
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

var _ folderLoader = (*folderLoaderMock)(nil)

type folderLoaderMock struct {
	mock.Mock
}

func (m *folderLoaderMock) Load(
	ctx context.Context, rootPath string,
	templateContext FolderLoaderTemplateContext,
) (res FolderLoaderResult, err error) {
	args := m.Called(ctx, rootPath, templateContext)
	return args.Get(0).(FolderLoaderResult), args.Error(1)
}
