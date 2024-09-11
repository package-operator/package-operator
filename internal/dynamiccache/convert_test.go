package dynamiccache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"package-operator.run/internal/testutil"
)

const name = "foo"

func TestEnsureUnstructured(t *testing.T) {
	t.Parallel()

	// Passing a secret object yields an unstructured with the same data.
	uns, wasConverted, err := ensureUnstructured(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, testutil.NewTestSchemeWithCoreV1())
	require.NoError(t, err)
	assert.IsType(t, &unstructured.Unstructured{}, uns)
	assert.True(t, wasConverted)

	// Assert that typemeta was carried over.
	assert.Equal(t, "v1", uns.Object["apiVersion"])
	assert.Equal(t, "Secret", uns.Object["kind"])

	// Assert that name was carried over.
	actualName := uns.Object["metadata"].(map[string]any)["name"].(string)
	assert.Equal(t, name, actualName)

	// Passing an unstructed object yiels the same unstructured.
	in := &unstructured.Unstructured{}
	uns, wasConverted, err = ensureUnstructured(in, testutil.NewTestSchemeWithCoreV1())
	require.NoError(t, err)
	assert.IsType(t, &unstructured.Unstructured{}, uns)
	assert.False(t, wasConverted)
	assert.Same(t, in, uns)
}

func TestEnsureUnstructuredList(t *testing.T) {
	t.Parallel()

	// Passing a secret object list yields an unstructured with the same data.
	uns, wasConverted, err := ensureUnstructuredList(&v1.SecretList{
		Items: []v1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			},
		},
	}, testutil.NewTestSchemeWithCoreV1())
	require.NoError(t, err)
	assert.IsType(t, &unstructured.UnstructuredList{}, uns)
	assert.True(t, wasConverted)

	// Assert that typemeta was carried over.
	assert.Equal(t, "v1", uns.Object["apiVersion"])
	assert.Equal(t, "SecretList", uns.Object["kind"])

	// Assert that name was carried over.
	actualName := uns.Object["items"].([]any)[0].(map[string]any)["metadata"].(map[string]any)["name"].(string)
	assert.Equal(t, name, actualName)

	// Passing an unstructed object yiels the same unstructured.
	in := &unstructured.UnstructuredList{}
	uns, wasConverted, err = ensureUnstructuredList(in, testutil.NewTestSchemeWithCoreV1())
	require.NoError(t, err)
	assert.IsType(t, &unstructured.UnstructuredList{}, uns)
	assert.False(t, wasConverted)
	assert.Same(t, in, uns)
}

func TestToStructured(t *testing.T) {
	t.Parallel()

	uns := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name": name,
		},
	}}
	secret := &v1.Secret{}
	err := toStructured(uns, secret)

	require.NoError(t, err)
	assert.Equal(t, name, secret.Name)
}

func TestToStructuredList_DataInMap(t *testing.T) {
	t.Parallel()

	uns := &unstructured.UnstructuredList{Object: map[string]any{
		"items": []any{
			map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name": name,
				},
			},
		},
	}}
	secretList := &v1.SecretList{}
	err := toStructuredList(uns, secretList)

	require.NoError(t, err)
	require.Len(t, secretList.Items, 1)
	assert.Equal(t, name, secretList.Items[0].Name)
}

func TestToStructuredList_DataInItemsField(t *testing.T) {
	t.Parallel()

	uns := &unstructured.UnstructuredList{
		Object: map[string]any{},
		Items: []unstructured.Unstructured{
			{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name": name,
					},
				},
			},
		},
	}
	secretList := &v1.SecretList{}
	err := toStructuredList(uns, secretList)

	require.NoError(t, err)
	require.Len(t, secretList.Items, 1)
	assert.Equal(t, name, secretList.Items[0].Name)
}
