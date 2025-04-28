package objectsets

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
)

func TestObjectSliceLoadReconciler(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()

	r := newObjectSliceLoadReconciler(testScheme, c, adapters.NewObjectSlice)

	object1 := corev1alpha1.ObjectSetObject{
		Object: unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "o-1",
				},
			},
		},
	}

	object2 := corev1alpha1.ObjectSetObject{
		Object: unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "o-2",
				},
			},
		},
	}

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test-ns",
			},
			Spec: corev1alpha1.ObjectSetSpec{
				ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
					Phases: []corev1alpha1.ObjectSetTemplatePhase{
						{
							Objects: []corev1alpha1.ObjectSetObject{
								object1,
							},
							Slices: []string{
								"slice-1",
							},
						},
					},
				},
			},
		},
	}

	var slice *corev1alpha1.ObjectSlice
	c.
		On("Get", mock.Anything, client.ObjectKey{
			Name:      "slice-1",
			Namespace: "test-ns",
		}, mock.AnythingOfType("*v1alpha1.ObjectSlice"), mock.Anything).
		Run(func(args mock.Arguments) {
			slice = args.Get(2).(*corev1alpha1.ObjectSlice)
			slice.Name = "slice-1"
			slice.Namespace = "test-ns"
			slice.Objects = []corev1alpha1.ObjectSetObject{
				object2,
			}
		}).
		Return(nil)

	c.
		On("Update", mock.Anything, mock.AnythingOfType("*v1alpha1.ObjectSlice"), mock.Anything).
		Return(nil)

	ctx := logr.NewContext(t.Context(), testr.New(t))
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)
	require.True(t, res.IsZero())

	assert.Equal(t, []metav1.OwnerReference{
		{
			APIVersion: "package-operator.run/v1alpha1",
			Kind:       "ObjectSet",
			Name:       "test",
		},
	}, slice.OwnerReferences)
	assert.Equal(t, []corev1alpha1.ObjectSetObject{
		object1, object2,
	}, objectSet.Spec.Phases[0].Objects)
}
