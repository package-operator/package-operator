package bootstrap

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/packages/packagecontent"
	"package-operator.run/package-operator/internal/packages/packageloader"
	"package-operator.run/package-operator/internal/testutil"
)

func Test_initializer_ensureClusterPackage(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &initializer{client: c}

	c.On("Create", mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(nil)

	pkg, err := b.ensureClusterPackage(ctx)
	require.NoError(t, err)
	assert.NotNil(t, pkg)
	c.AssertExpectations(t)
}

func Test_initializer_ensureClusterPackage_alreadyExists(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &initializer{client: c}

	c.On("Create", mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything).
		Return(errors.NewAlreadyExists(schema.GroupResource{}, ""))
	c.On("Patch", mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterPackage"), mock.Anything, mock.Anything).
		Return(nil)

	pkg, err := b.ensureClusterPackage(ctx)
	require.NoError(t, err)
	assert.NotNil(t, pkg)
	c.AssertExpectations(t)
}

func Test_initializer_ensureCRDs(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &initializer{client: c}

	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))

	c.On("Create", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Return(errors.NewAlreadyExists(schema.GroupResource{}, ""))
	c.On("Create", mock.Anything, mock.Anything, mock.Anything).
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

func Test_initializer_crdsFromPackage(t *testing.T) {
	t.Parallel()
	l := &loaderMock{}
	ctx := logr.NewContext(context.Background(), testr.New(t))

	b := &initializer{
		loader: l,
		pullImage: func(ctx context.Context, path string) (
			packagecontent.Files, error,
		) {
			return nil, nil
		},
	}

	crd := unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGK.WithVersion("v1"))
	crd.SetAnnotations(map[string]string{
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
				"": {crd},
			},
		}, nil)

	crds, err := b.crdsFromPackage(ctx)
	require.NoError(t, err)
	assert.Len(t, crds, 1)
	l.AssertExpectations(t)
}

func Test_crdsFromTemplateSpec(t *testing.T) {
	t.Parallel()
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
