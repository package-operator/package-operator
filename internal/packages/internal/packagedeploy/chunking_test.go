package packagedeploy

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
)

func Test_determineChunkingStrategyForPackage(t *testing.T) {
	t.Parallel()

	t.Run("EachObject", func(t *testing.T) {
		t.Parallel()

		pkg := &adapters.GenericPackage{
			Package: corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						chunkingStrategyAnnotation: string(chunkingStrategyEachObject),
					},
				},
			},
		}
		c := determineChunkingStrategyForPackage(pkg)
		_, ok := c.(*EachObjectChunker)
		assert.True(t, ok)
	})

	t.Run("Default", func(t *testing.T) {
		t.Parallel()

		pkg := &adapters.GenericPackage{
			Package: corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}
		c := determineChunkingStrategyForPackage(pkg)
		_, ok := c.(*NoOpChunker)
		assert.True(t, ok)
	})
}

func TestNoOpChunker(t *testing.T) {
	t.Parallel()

	ctx := logr.NewContext(context.Background(), testr.New(t))

	c := &NoOpChunker{}
	chunks, err := c.Chunk(ctx, &corev1alpha1.ObjectSetTemplatePhase{})
	require.NoError(t, err)
	assert.Nil(t, chunks)
}

func TestEachObjectChunker(t *testing.T) {
	t.Parallel()

	ctx := logr.NewContext(context.Background(), testr.New(t))

	c := &EachObjectChunker{}
	chunks, err := c.Chunk(ctx, &corev1alpha1.ObjectSetTemplatePhase{
		Objects: []corev1alpha1.ObjectSetObject{
			{}, {}, {},
		},
	})
	require.NoError(t, err)
	assert.Len(t, chunks, 3)
}

func TestBinpackNextFitChunkerChunker(t *testing.T) {
	t.Parallel()

	tcases := []struct {
		name                string
		objectSizes         []int
		expectedBucketCount int
	}{
		{
			name: "two small objects - not filling up a single bucket - bypassing chunking",
			objectSizes: []int{
				10 * 1024,
				10 * 1024,
			},
			expectedBucketCount: 0,
		},
		{
			name: "one big two small",
			objectSizes: []int{
				1024 * 1024,
				10 * 1024,
				10 * 1024,
			},
			expectedBucketCount: 2,
		},
		{
			name: "one small one big one small",
			objectSizes: []int{
				10 * 1024,
				1024 * 1024,
				10 * 1024,
			},
			expectedBucketCount: 3,
		},
		{
			name: "three big",
			objectSizes: []int{
				1024 * 1024,
				1024 * 1024,
				1024 * 1024,
			},
			expectedBucketCount: 3,
		},
		{
			name: "three bigger",
			objectSizes: []int{
				1025 * 1024,
				1025 * 1024,
				1025 * 1024,
			},
			expectedBucketCount: 3,
		},
	}

	for i := range tcases {
		tc := tcases[i]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := logr.NewContext(context.Background(), testr.New(t))

			c := &BinpackNextFitChunker{}

			objects := make([]corev1alpha1.ObjectSetObject, 0, len(tc.objectSizes))
			for _, size := range tc.objectSizes {
				objects = append(objects, corev1alpha1.ObjectSetObject{
					Object: genBigObject(size),
				})
			}

			chunks, err := c.Chunk(ctx, &corev1alpha1.ObjectSetTemplatePhase{
				Objects: objects,
			})
			require.NoError(t, err)
			assert.Len(t, chunks, tc.expectedBucketCount)
		})
	}
}

func genBigObject(size int) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetAnnotations(map[string]string{
		"a": strings.Repeat("a", size),
	})
	return obj
}
