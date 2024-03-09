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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/packages/internal/packagetypes"
	"package-operator.run/internal/testutil"
)

var (
	_          deploymentReconciler = (*deploymentReconcilerMock)(nil)
	errExample                      = errors.New("example error")
	testDgst                        = "sha256:52a6b1268e32ed5b6f59da8222f7627979bfb739f32aae3fb5b5ed31b8bf80c4"
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

func TestPackageDeployer_Deploy(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	structuralLoaderMock := &structuralLoaderMock{}
	deploymentReconcilerMock := &deploymentReconcilerMock{}

	l := &PackageDeployer{
		client: c,
		scheme: testScheme,

		newObjectDeployment: adapters.NewObjectDeployment,
		structuralLoader:    structuralLoaderMock,

		deploymentReconciler: deploymentReconcilerMock,
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))

	obj1 := unstructured.Unstructured{Object: map[string]any{}}
	obj1.SetAnnotations(map[string]string{
		manifests.PackagePhaseAnnotation: "phase-1",
	})

	structuralLoaderMock.
		On("LoadComponent", mock.Anything, mock.Anything, mock.Anything).
		Return(&packagetypes.Package{
			Manifest: &manifests.PackageManifest{
				Spec: manifests.PackageManifestSpec{
					Scopes: []manifests.PackageManifestScope{
						manifests.PackageManifestScopeNamespaced,
					},
					Phases: []manifests.PackageManifestPhase{
						{
							Name: "phase-1",
						},
						{
							Name: "empty-phase",
						},
					},
					Images: []manifests.PackageManifestImage{
						{Name: "nginx", Image: "nginx:1.23.3"},
					},
				},
			},
			ManifestLock: &manifests.PackageManifestLock{
				Spec: manifests.PackageManifestLockSpec{
					Images: []manifests.PackageManifestLockImage{
						{Name: "nginx", Image: "nginx:1.23.3", Digest: testDgst},
					},
				},
			},
		}, nil)

	deploymentReconcilerMock.
		On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	apiPkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", Namespace: "test",
			},
		},
	}
	rawPkg := &packagetypes.RawPackage{
		Files: packagetypes.Files{},
	}
	err := l.Deploy(ctx, apiPkg, rawPkg, manifests.PackageEnvironment{})
	require.NoError(t, err)

	packageInvalid := meta.FindStatusCondition(apiPkg.Status.Conditions, corev1alpha1.PackageInvalid)
	assert.Nil(t, packageInvalid, "Invalid condition should not be reported")
}

func TestPackageDeployer_Deploy_Error(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	structuralLoaderMock := &structuralLoaderMock{}
	deploymentReconcilerMock := &deploymentReconcilerMock{}

	l := &PackageDeployer{
		client: c,
		scheme: testScheme,

		newObjectDeployment: adapters.NewObjectDeployment,
		structuralLoader:    structuralLoaderMock,

		deploymentReconciler: deploymentReconcilerMock,
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))

	structuralLoaderMock.
		On("LoadComponent", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errExample)

	obj1 := unstructured.Unstructured{Object: map[string]any{}}
	obj1.SetAnnotations(map[string]string{
		manifests.PackagePhaseAnnotation: "phase-1",
	})
	apiPkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", Namespace: "test",
			},
		},
	}
	rawPkg := &packagetypes.RawPackage{
		Files: packagetypes.Files{},
	}
	err := l.Deploy(ctx, apiPkg, rawPkg, manifests.PackageEnvironment{})
	require.NoError(t, err)

	packageInvalid := meta.FindStatusCondition(apiPkg.Status.Conditions, corev1alpha1.PackageInvalid)
	if assert.NotNil(t, packageInvalid) {
		assert.Equal(t, metav1.ConditionTrue, packageInvalid.Status)
		assert.Equal(t, "LoadError", packageInvalid.Reason)
		assert.Equal(t, "example error", packageInvalid.Message)
	}
}

func TestImageWithDigestOk(t *testing.T) {
	t.Parallel()
	//nolint: goconst // Better to read that way.
	tests := []struct {
		image  string
		digest string
		expOut string
	}{
		{"nginx", testDgst, "index.docker.io/library/nginx@" + testDgst},
		{"nginx@" + testDgst, testDgst, "index.docker.io/library/nginx@" + testDgst},
		{"nginx:1.23.3", testDgst, "index.docker.io/library/nginx@" + testDgst},
		{"nginx:1.23.3@" + testDgst, testDgst, "index.docker.io/library/nginx@" + testDgst},
		{"jboss/keycloak", testDgst, "index.docker.io/jboss/keycloak@" + testDgst},
		{"jboss/keycloak@" + testDgst, testDgst, "index.docker.io/jboss/keycloak@" + testDgst},
		{"jboss/keycloak:16.1.1", testDgst, "index.docker.io/jboss/keycloak@" + testDgst},
		{"jboss/keycloak:16.1.1@" + testDgst, testDgst, "index.docker.io/jboss/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak", testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak@" + testDgst, testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak:20.0.3", testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"quay.io/keycloak/keycloak:20.0.3@" + testDgst, testDgst, "quay.io/keycloak/keycloak@" + testDgst},
		{"example.com:12345/imggroup/imgname", testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
		{"example.com:12345/imggroup/imgname@" + testDgst, testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
		{"example.com:12345/imggroup/imgname:1.0.0", testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
		{"example.com:12345/imggroup/imgname:1.0.0@" + testDgst, testDgst, "example.com:12345/imggroup/imgname@" + testDgst},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.image, func(t *testing.T) {
			t.Parallel()

			out, err := ImageWithDigest(test.image, test.digest)
			require.NoError(t, err)
			require.Equal(t, test.expOut, out)
		})
	}
}

func TestImageWithDigestError(t *testing.T) {
	t.Parallel()

	_, err := ImageWithDigest("", testDgst)
	require.Error(t, err)
}

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

type structuralLoaderMock struct {
	mock.Mock
}

func (m *structuralLoaderMock) LoadComponent(
	ctx context.Context, rawPkg *packagetypes.RawPackage, componentName string,
) (*packagetypes.Package, error) {
	args := m.Called(ctx, rawPkg, componentName)
	pkg, _ := args.Get(0).(*packagetypes.Package)
	return pkg, args.Error(1)
}
