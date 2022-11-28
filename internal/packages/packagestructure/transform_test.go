package packagestructure

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
)

var errExample = errors.New("example error")

func TestTransformerList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		t1 := &TransformerMock{}
		t2 := &TransformerMock{}

		t1.On("Transform", mock.Anything, mock.Anything).Return(nil)
		t2.On("Transform", mock.Anything, mock.Anything).Return(nil)

		tl := TransformerList{
			t1, t2,
		}

		ctx := context.Background()
		err := tl.Transform(ctx, nil)
		require.NoError(t, err)

		t1.AssertCalled(t, "Transform", mock.Anything, mock.Anything)
		t2.AssertCalled(t, "Transform", mock.Anything, mock.Anything)
	})

	t.Run("stops at first error", func(t *testing.T) {
		t1 := &TransformerMock{}
		t2 := &TransformerMock{}

		t1.On("Transform", mock.Anything, mock.Anything).
			Return(errExample)
		t2.On("Transform", mock.Anything, mock.Anything).Return(nil)

		tl := TransformerList{
			t1, t2,
		}

		ctx := context.Background()
		err := tl.Transform(ctx, nil)
		assert.EqualError(t, err, errExample.Error())

		t1.AssertCalled(t, "Transform", mock.Anything, mock.Anything)
		t2.AssertNotCalled(t, "Transform", mock.Anything, mock.Anything)
	})
}

var (
	_ Transformer = (*TransformerMock)(nil)
)

type TransformerMock struct {
	mock.Mock
}

func (m *TransformerMock) Transform(ctx context.Context, packageContent *PackageContent) error {
	args := m.Called(ctx, packageContent)
	return args.Error(0)
}

func TestCommonObjectLabelsTransformer(t *testing.T) {
	colt := &CommonObjectLabelsTransformer{
		Package: &metav1.ObjectMeta{
			Name: "sepp",
		},
	}

	packageContent := &PackageContent{
		PackageManifest: &manifestsv1alpha1.PackageManifest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-cool-pkg",
			},
		},
		Manifests: ManifestMap{
			"test.yaml": []unstructured.Unstructured{
				{},
			},
		},
	}

	ctx := context.Background()
	err := colt.Transform(ctx, packageContent)
	require.NoError(t, err)
	obj := packageContent.Manifests["test.yaml"][0]
	assert.Equal(t, map[string]string{
		manifestsv1alpha1.PackageInstanceLabel: "sepp",
		manifestsv1alpha1.PackageLabel:         "my-cool-pkg",
	}, obj.GetLabels())
}
