package dynamiccache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_indexFuncForExtractor(t *testing.T) {
	const indexedMetadataKey = "my-customer-index"
	ifn := indexFuncForExtractor(
		"my-cool-field", func(o client.Object) []string {
			return []string{
				o.GetAnnotations()[indexedMetadataKey],
			}
		},
	)

	tests := []struct {
		name              string
		obj               client.Object
		expectedIndexKeys []string
	}{
		{
			name: "namespaced",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Annotations: map[string]string{
						indexedMetadataKey: "test42",
					},
				},
			},
			expectedIndexKeys: []string{
				"test/test42",
				"__all_namespaces/test42",
			},
		},
		{
			name: "cluster-scoped",
			obj: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						indexedMetadataKey: "test42",
					},
				},
			},
			expectedIndexKeys: []string{
				"__all_namespaces/test42",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vals, err := ifn(test.obj)
			require.NoError(t, err)
			assert.Equal(t, test.expectedIndexKeys, vals)
		})
	}

}
