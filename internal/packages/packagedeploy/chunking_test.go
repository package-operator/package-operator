package packagedeploy

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func Test_determineChunkingStrategyForPackage(t *testing.T) {
	t.Run("EachObject", func(t *testing.T) {
		t.Parallel()

		pkg := &GenericPackage{
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

		pkg := &GenericPackage{
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
