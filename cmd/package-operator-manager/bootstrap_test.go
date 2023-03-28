package main

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageloader"
	"package-operator.run/package-operator/internal/testutil"
)

func TestBootstrapper_BootstrapJustPatches(t *testing.T) {
	c := testutil.NewClient()
	log := testr.New(t)
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &bootstrapper{client: c, log: log}

	c.
		On("Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Run(func(args mock.Arguments) {
			pkg := args.Get(2).(*corev1alpha1.ClusterPackage)
			pkg.Status.Conditions = []metav1.Condition{
				{Type: corev1alpha1.PackageAvailable, Status: metav1.ConditionTrue},
			}
		}).
		Return(nil)
	c.
		On("Patch", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"),
			mock.Anything, mock.Anything).
		Return(nil)

	err := b.Bootstrap(ctx)
	require.NoError(t, err)

	c.AssertExpectations(t)
}

func TestBootstrapper_BootstrapSelfBootstrap(t *testing.T) {
	c := testutil.NewClient()
	log := testr.New(t)
	ctx := logr.NewContext(context.Background(), testr.New(t))
	l := &loaderMock{}

	var runManagerCalled bool
	runManager := func(ctx context.Context) error {
		runManagerCalled = true
		return nil
	}

	b := &bootstrapper{
		client: c, log: log,
		loadFiles: func(ctx context.Context, path string) (packagecontent.Files, error) {
			return nil, nil
		},
		loader:     l,
		runManager: runManager,
	}

	crdObj := unstructured.Unstructured{}
	crdObj.SetGroupVersionKind(crdGK.WithVersion("v1"))
	crdObj.SetAnnotations(map[string]string{
		manifestsv1alpha1.PackagePhaseAnnotation: "test",
	})
	l.On("FromFiles", mock.Anything, mock.Anything, mock.Anything).
		Return(&packagecontent.Package{
			PackageManifest: &manifestsv1alpha1.PackageManifest{
				Spec: manifestsv1alpha1.PackageManifestSpec{
					Phases: []manifestsv1alpha1.PackageManifestPhase{
						{Name: "test"},
					},
				},
			},
			Objects: map[string][]unstructured.Unstructured{
				"test.yaml": {crdObj},
			},
		}, nil)

	c.
		On("Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Once().
		Return(nil)
	c.
		On("Create", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Once().
		Return(nil)
	c.
		On("Create", mock.Anything,
			mock.AnythingOfType("*unstructured.Unstructured"), mock.Anything).
		Once().
		Return(nil)
	c.
		On("Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Run(func(args mock.Arguments) {
			pkg := args.Get(2).(*corev1alpha1.ClusterPackage)
			pkg.Status.Conditions = []metav1.Condition{
				{Type: corev1alpha1.PackageAvailable, Status: metav1.ConditionTrue},
			}
		}).
		Return(nil)

	err := b.Bootstrap(ctx)
	require.NoError(t, err)

	assert.True(t, runManagerCalled)
	c.AssertExpectations(t)
	l.AssertExpectations(t)
}

func TestBootstrapper_cancelWhenPackageAvailable(t *testing.T) {
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &bootstrapper{client: c}

	c.
		On("Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Run(func(args mock.Arguments) {
			pkg := args.Get(2).(*corev1alpha1.ClusterPackage)
			pkg.Status.Conditions = []metav1.Condition{
				{Type: corev1alpha1.PackageAvailable, Status: metav1.ConditionTrue},
			}
		}).
		Return(nil)

	var cancelCalled bool
	cancel := func() { cancelCalled = true }

	b.cancelWhenPackageAvailable(ctx, cancel)
	assert.True(t, cancelCalled)
}

func TestBootstrapper_isPackageAvailable(t *testing.T) {
	t.Run("unavailable", func(t *testing.T) {
		c := testutil.NewClient()
		ctx := logr.NewContext(context.Background(), testr.New(t))

		b := &bootstrapper{client: c}

		c.
			On("Get", mock.Anything, mock.Anything,
				mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
			Return(nil)

		available, err := b.isPackageAvailable(ctx)
		require.NoError(t, err)
		assert.False(t, available)
		c.AssertExpectations(t)
	})

	t.Run("available", func(t *testing.T) {
		c := testutil.NewClient()
		ctx := logr.NewContext(context.Background(), testr.New(t))

		b := &bootstrapper{client: c}

		c.
			On("Get", mock.Anything, mock.Anything,
				mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
			Run(func(args mock.Arguments) {
				pkg := args.Get(2).(*corev1alpha1.ClusterPackage)
				pkg.Status.Conditions = []metav1.Condition{
					{Type: corev1alpha1.PackageAvailable, Status: metav1.ConditionTrue},
				}
			}).
			Return(nil)

		available, err := b.isPackageAvailable(ctx)
		require.NoError(t, err)
		assert.True(t, available)
		c.AssertExpectations(t)
	})
}

func TestBootstrapper_createPKOPackage(t *testing.T) {
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &bootstrapper{client: c}

	c.
		On("Create", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(nil)

	pkg, err := b.createPKOPackage(ctx)
	require.NoError(t, err)
	assert.NotNil(t, pkg)
	c.AssertExpectations(t)
}

func TestBootstrapper_ensureCRDs(t *testing.T) {
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &bootstrapper{client: c}

	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))

	c.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Return(errors.NewAlreadyExists(schema.GroupResource{}, ""))
	c.
		On("Create", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	crds := []unstructured.Unstructured{crd, crd}
	err := b.ensureCRDs(ctx, crds)
	require.NoError(t, err)

	for _, crd := range crds {
		assert.Equal(t, map[string]string{
			controllers.DynamicCacheLabel: "True",
		}, crd.GetLabels())
	}
	c.AssertExpectations(t)
}

func Test_crdsFromTemplateSpec(t *testing.T) {
	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))

	ts := corev1alpha1.ObjectSetTemplateSpec{
		Phases: []corev1alpha1.ObjectSetTemplatePhase{
			{
				Objects: []corev1alpha1.ObjectSetObject{
					{
						Object: unstructured.Unstructured{},
					},
				},
			},
			{
				Objects: []corev1alpha1.ObjectSetObject{
					{
						Object: crd,
					},
				},
			},
		},
	}
	crds := crdsFromTemplateSpec(ts)
	assert.Len(t, crds, 1)
}

type loaderMock struct {
	mock.Mock
}

func (m *loaderMock) FromFiles(
	ctx context.Context, files packagecontent.Files,
	opts ...packageloader.Option,
) (*packagecontent.Package, error) {
	args := m.Called(ctx, files, opts)
	return args.Get(0).(*packagecontent.Package), args.Error(1)
}
