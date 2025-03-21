package packagedeploy

import (
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
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/constants"
	"package-operator.run/internal/testutil"
)

func Test_DeploymentReconciler_Reconcile(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	r := newDeploymentReconciler(testScheme, c,
		adapters.NewObjectDeployment,
		adapters.NewObjectSlice,
		adapters.NewObjectSliceList,
		newGenericObjectSetList)
	ctx := logr.NewContext(t.Context(), testr.New(t))

	deploy := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-depl",
			},
			Spec: corev1alpha1.ObjectDeploymentSpec{
				Template: corev1alpha1.ObjectSetTemplate{
					Spec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name: "test",
								Objects: []corev1alpha1.ObjectSetObject{
									{
										Object: unstructured.Unstructured{},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	c.
		On("Get",
			mock.Anything,
			mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectDeployment"),
			mock.Anything,
		).
		Once().
		Return(errors.NewNotFound(schema.GroupResource{}, ""))
	var createdDeployment *corev1alpha1.ObjectDeployment
	c.
		On("Create", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectDeployment"),
			mock.Anything).
		Run(func(args mock.Arguments) {
			createdDeployment = args.Get(1).(*corev1alpha1.ObjectDeployment).DeepCopy()
		}).
		Return(nil)
	var createdSlice *corev1alpha1.ObjectSlice
	c.
		On("Create", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectSlice"),
			mock.Anything).
		Run(func(args mock.Arguments) {
			createdSlice = args.Get(1).(*corev1alpha1.ObjectSlice).DeepCopy()
		}).
		Return(nil)

		// retries on conflict
	c.
		On("Update", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectDeployment"),
			mock.Anything).
		Once().
		Return(errors.NewConflict(schema.GroupResource{}, "", nil))
	c.
		On("Get",
			mock.Anything,
			mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectDeployment"),
			mock.Anything,
		).
		Return(nil)

	var updatedDeployment *corev1alpha1.ObjectDeployment
	c.
		On("Update", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectDeployment"),
			mock.Anything).
		Run(func(args mock.Arguments) {
			updatedDeployment = args.Get(1).(*corev1alpha1.ObjectDeployment).DeepCopy()
		}).
		Return(nil)
	c.
		On("List", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectSetList"),
			mock.Anything).
		Return(nil)
	c.
		On("List", mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectSliceList"),
			mock.Anything).
		Return(nil)

	err := r.Reconcile(ctx, deploy, &EachObjectChunker{})
	require.NoError(t, err)

	// ObjectDeployment is created empty.
	assert.Empty(t, createdDeployment.Spec.Template.Spec.Phases)

	assert.Equal(t, []corev1alpha1.ObjectSetObject{
		{
			Object: unstructured.Unstructured{},
		},
	}, createdSlice.Objects)

	assert.Equal(t, []corev1alpha1.ObjectSetTemplatePhase{
		{
			Name:   "test",
			Slices: []string{"test-depl-95565fb5c"},
		},
	}, updatedDeployment.Spec.Template.Spec.Phases)
}

func TestDeploymentReconciler_reconcileSlice_hashCollision(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	r := newDeploymentReconciler(testScheme, c,
		adapters.NewObjectDeployment,
		adapters.NewObjectSlice,
		adapters.NewObjectSliceList,
		newGenericObjectSetList)
	ctx := logr.NewContext(t.Context(), testr.New(t))

	deploy := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-depl",
			},
		},
	}

	slice := &adapters.ObjectSlice{
		ObjectSlice: corev1alpha1.ObjectSlice{
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: unstructured.Unstructured{},
				},
			},
		},
	}

	c.On("Create",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.ObjectSlice"),
		mock.Anything).
		Once().
		Return(errors.NewAlreadyExists(schema.GroupResource{}, ""))

	c.On("Create",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.ObjectSlice"),
		mock.Anything).
		Return(nil)

	c.On("Get",
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.ObjectSlice"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			slice := args.Get(2).(*corev1alpha1.ObjectSlice)
			*slice = corev1alpha1.ObjectSlice{
				Objects: []corev1alpha1.ObjectSetObject{
					{
						Object: unstructured.Unstructured{},
					},
				},
			}
		}).
		Return(nil)

	err := r.reconcileSlice(ctx, deploy, slice)
	require.NoError(t, err)

	c.AssertNumberOfCalls(t, "Create", 2)
}

func TestDeploymentReconciler_sliceGarbageCollection(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	r := newDeploymentReconciler(testScheme, c,
		adapters.NewObjectDeployment,
		adapters.NewObjectSlice,
		adapters.NewObjectSliceList,
		newGenericObjectSetList)
	ctx := logr.NewContext(t.Context(), testr.New(t))

	deploy := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-depl",
			},
			Spec: corev1alpha1.ObjectDeploymentSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test": "test",
					},
				},
				Template: corev1alpha1.ObjectSetTemplate{
					Spec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Slices: []string{"slice0-xxx"},
							},
						},
					},
				},
			},
		},
	}

	objectSet1 := &corev1alpha1.ObjectSet{
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:   "test",
						Slices: []string{"slice1-xxx"},
					},
				},
			},
		},
	}

	objectSlice0 := &corev1alpha1.ObjectSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slice0-xxx",
		},
	}
	objectSlice1 := &corev1alpha1.ObjectSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slice1-xxx",
		},
	}
	objectSlice2 := &corev1alpha1.ObjectSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slice2-xxx",
		},
	}

	c.
		On("List",
			mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectSetList"),
			mock.Anything).
		Run(func(args mock.Arguments) {
			list := args.Get(1).(*corev1alpha1.ObjectSetList)
			list.Items = []corev1alpha1.ObjectSet{
				*objectSet1,
			}
		}).
		Return(nil)
	c.
		On("List",
			mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectSliceList"),
			mock.Anything).
		Run(func(args mock.Arguments) {
			list := args.Get(1).(*corev1alpha1.ObjectSliceList)
			list.Items = []corev1alpha1.ObjectSlice{
				*objectSlice0, *objectSlice1, *objectSlice2,
			}
		}).
		Return(nil)
	c.
		On("Delete",
			mock.Anything,
			mock.AnythingOfType("*v1alpha1.ObjectSlice"),
			mock.Anything).
		Return(nil)

	err := r.sliceGarbageCollection(ctx, deploy)
	require.NoError(t, err)

	c.AssertNumberOfCalls(t, "Delete", 1)
	c.AssertCalled(
		t, "Delete", mock.Anything, objectSlice2, mock.Anything)
}

func Test_sliceCollisionError(t *testing.T) {
	t.Parallel()

	e := &sliceCollisionError{
		key: client.ObjectKey{
			Name: "test", Namespace: "test",
		},
	}

	assert.Equal(t, "ObjectSlice collision with test/test", e.Error())
}

func Test_getChangeCause(t *testing.T) {
	t.Parallel()
	const deploy1Cause = "Aaaaaaaaah!"
	deploy1 := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test1",
				Annotations: map[string]string{
					manifestsv1alpha1.PackageSourceImageAnnotation: "quay.io/xxx/some:thing",
					manifestsv1alpha1.PackageConfigAnnotation:      "{}",
					constants.ChangeCauseAnnotation:                deploy1Cause,
				},
			},
		},
	}

	// Different Image
	deploy2 := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test2",
				Annotations: map[string]string{
					manifestsv1alpha1.PackageSourceImageAnnotation: "quay.io/xxx/some2:thing",
					manifestsv1alpha1.PackageConfigAnnotation:      "{}",
				},
			},
		},
	}

	// Different Config
	deploy3 := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test3",
				Annotations: map[string]string{
					manifestsv1alpha1.PackageSourceImageAnnotation: "quay.io/xxx/some:thing",
					manifestsv1alpha1.PackageConfigAnnotation:      "{xxx}",
				},
			},
		},
	}

	// Both Different
	deploy4 := &adapters.ObjectDeployment{
		ObjectDeployment: corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test4",
				Annotations: map[string]string{
					manifestsv1alpha1.PackageSourceImageAnnotation: "quay.io/xxx/some3:thing",
					manifestsv1alpha1.PackageConfigAnnotation:      "{xxx}",
				},
			},
		},
	}

	tests := []struct {
		name              string
		actualDeployment  adapters.ObjectDeploymentAccessor
		desiredDeployment adapters.ObjectDeploymentAccessor
		expected          string
	}{
		{
			name:              "no change",
			actualDeployment:  deploy1,
			desiredDeployment: deploy1,
			expected:          deploy1Cause,
		},
		{
			name:              "image change",
			actualDeployment:  deploy1,
			desiredDeployment: deploy2,
			expected:          "Package source image changed.",
		},
		{
			name:              "config change",
			actualDeployment:  deploy1,
			desiredDeployment: deploy3,
			expected:          "Package config changed.",
		},
		{
			name:              "both change",
			actualDeployment:  deploy1,
			desiredDeployment: deploy4,
			expected:          "Package source image and config changed.",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cause := getChangeCause(test.actualDeployment, test.desiredDeployment)
			assert.Equal(t, test.expected, cause)
		})
	}
}
