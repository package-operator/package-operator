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
	testDgst                        = "sha256:52a6b1268e32ed5b6f59da8222f7627979bfb739f32aae3fb5b5ed31b8bf80c4"
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
		Run(func(args mock.Arguments) {
			pkg := args.Get(2).(*corev1alpha1.Package)
			pkg.Status.Conditions = []metav1.Condition{
				// This condition is supposed to be removed:
				{Type: corev1alpha1.PackageInvalid, Status: metav1.ConditionTrue},
			}
		}).
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
				Images: []manifestsv1alpha1.PackageManifestImage{
					{Name: "nginx", Image: "nginx:1.23.3"},
				},
			},
		},
		PackageManifestLock: &manifestsv1alpha1.PackageManifestLock{
			Spec: manifestsv1alpha1.PackageManifestLockSpec{
				Images: []manifestsv1alpha1.PackageManifestLockImage{
					{Name: "nginx", Image: "nginx:1.23.3", Digest: testDgst},
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
	assert.Nil(t, packageInvalid, "Invalid condition should not be reported")
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

func TestImageWithDigestOk(t *testing.T) {
	tests := []struct {
		image  string
		digest string
		expOut string
	}{
		{"nginx", testDgst, "docker.io/library/nginx@" + testDgst},
		{"nginx@" + testDgst, testDgst, "docker.io/library/nginx@" + testDgst},
		{"nginx:1.23.3", testDgst, "docker.io/library/nginx@" + testDgst},
		{"nginx:1.23.3@" + testDgst, testDgst, "docker.io/library/nginx@" + testDgst},
		{"jboss/keycloak", testDgst, "docker.io/jboss/keycloak@" + testDgst},
		{"jboss/keycloak@" + testDgst, testDgst, "docker.io/jboss/keycloak@" + testDgst},
		{"jboss/keycloak:16.1.1", testDgst, "docker.io/jboss/keycloak@" + testDgst},
		{"jboss/keycloak:16.1.1@" + testDgst, testDgst, "docker.io/jboss/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak", testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak@" + testDgst, testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak:20.0.3", testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak:20.0.3@" + testDgst, testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"example.com:12345/imggroup/imgname", testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
		{"example.com:12345/imggroup/imgname@" + testDgst, testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
		{"example.com:12345/imggroup/imgname:1.0.0", testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
		{"example.com:12345/imggroup/imgname:1.0.0@" + testDgst, testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
	}

	for _, test := range tests {
		t.Run(test.image, func(t *testing.T) {
			out, err := ImageWithDigest(test.image, test.digest)
			require.NoError(t, err)
			require.Equal(t, test.expOut, out)
		})
	}
}

func TestImageWithDigestError(t *testing.T) {
	tests := []struct {
		image  string
		digest string
		expErr string
	}{
		{"", testDgst, "invalid reference format"},
		{"/imgname:latest", testDgst, "invalid reference format"},
		{"nginx", "", "invalid digest format"},
		{"nginx", "-_**", "invalid digest format"},
		{"nginx", "sha256:12345", "invalid digest format"},
	}

	for _, test := range tests {
		t.Run(test.image, func(t *testing.T) {
			_, err := ImageWithDigest(test.image, test.digest)
			require.ErrorContains(t, err, test.expErr)
		})
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
	ctx context.Context, path packagecontent.Files, opts ...packageloader.Option,
) (*packagecontent.Package, error) {
	args := m.Called(ctx, path, opts)
	return args.Get(0).(*packagecontent.Package), args.Error(1)
}
