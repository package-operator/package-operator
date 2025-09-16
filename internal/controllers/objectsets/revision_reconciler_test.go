package objectsets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func TestRevisionReconciler_DefaultRevision(t *testing.T) {
	t.Parallel()
	testClient := testutil.NewClient()
	testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r := &revisionReconciler{
		scheme:       testScheme,
		newObjectSet: adapters.NewObjectSet,
		client:       testClient,
	}

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)

	assert.True(t, res.IsZero(), "unexpected requeue")
	assert.Equal(t, int64(1), objectSet.Status.Revision)
}

func TestRevisionReconciler_FromPrevious(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()
	testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r := &revisionReconciler{
		scheme:       testScheme,
		newObjectSet: adapters.NewObjectSet,
		client:       testClient,
	}

	prev1 := &corev1alpha1.ObjectSet{
		Status: corev1alpha1.ObjectSetStatus{
			Revision: 14,
		},
	}
	testClient.
		On("Get", mock.Anything, client.ObjectKey{
			Name:      "prev1",
			Namespace: "xxx",
		}, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			out := args.Get(2).(*corev1alpha1.ObjectSet)
			*out = *prev1
		}).
		Return(nil)

	prev2 := &corev1alpha1.ObjectSet{
		Status: corev1alpha1.ObjectSetStatus{
			Revision: 4,
		},
	}
	testClient.
		On("Get", mock.Anything, client.ObjectKey{
			Name:      "prev2",
			Namespace: "xxx",
		}, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			out := args.Get(2).(*corev1alpha1.ObjectSet)
			*out = *prev2
		}).
		Return(nil)

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "xxx",
			},
			Spec: corev1alpha1.ObjectSetSpec{
				Previous: []corev1alpha1.PreviousRevisionReference{
					{
						Name: "prev1",
					},
					{
						Name: "prev2",
					},
				},
			},
		},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)

	assert.True(t, res.IsZero(), "unexpected requeue")
	assert.Equal(t, int64(15), objectSet.Status.Revision)
}

func TestRevisionReconciler_WaitForPrevious(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()
	testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r := &revisionReconciler{
		scheme:       testScheme,
		newObjectSet: adapters.NewObjectSet,
		client:       testClient,
	}

	prev1 := &corev1alpha1.ObjectSet{
		Status: corev1alpha1.ObjectSetStatus{
			// does not report Revision
		},
	}
	testClient.
		On("Get", mock.Anything, client.ObjectKey{
			Name:      "prev1",
			Namespace: "xxx",
		}, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			out := args.Get(2).(*corev1alpha1.ObjectSet)
			*out = *prev1
		}).
		Return(nil)

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "xxx",
			},
			Spec: corev1alpha1.ObjectSetSpec{
				Previous: []corev1alpha1.PreviousRevisionReference{
					{
						Name: "prev1",
					},
				},
			},
		},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)

	assert.Equal(t, revisionReconcilerRequeueDelay, res.RequeueAfter)
	assert.False(t, res.IsZero())
	assert.Equal(t, int64(0), objectSet.Status.Revision)
}

func TestRevisionReconciler_InvalidPreviousReference(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()
	testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r := &revisionReconciler{
		scheme:       testScheme,
		newObjectSet: adapters.NewObjectSet,
		client:       testClient,
	}

	prev := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prev",
			Namespace: "xxx",
		},
		Status: corev1alpha1.ObjectSetStatus{
			Revision: 42,
		},
	}
	testClient.
		On("Get", mock.Anything, client.ObjectKeyFromObject(prev), mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			out := args.Get(2).(*corev1alpha1.ObjectSet)
			*out = *prev
		}).
		Return(nil)

	prevNotFound := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prev-not-found",
			Namespace: "xxx",
		},
	}
	testClient.
		On("Get", mock.Anything, client.ObjectKeyFromObject(prevNotFound), mock.Anything, mock.Anything).
		Return(errors.NewNotFound(schema.GroupResource{}, prevNotFound.Name))

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "xxx",
			},
			Spec: corev1alpha1.ObjectSetSpec{
				Previous: []corev1alpha1.PreviousRevisionReference{
					{
						Name: prev.Name,
					},
					{
						Name: prevNotFound.Name,
					},
				},
			},
		},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)

	assert.True(t, res.IsZero(), "unexpected requeue")
	assert.Equal(t, prev.Status.Revision+1, objectSet.Status.Revision)

	testClient.AssertExpectations(t)
	testClient.StatusMock.AssertExpectations(t)
}

func TestRevisionReconciler_SetStatusFromSpec(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()

	r := &revisionReconciler{
		scheme:       testScheme,
		newObjectSet: adapters.NewObjectSet,
		client:       testClient,
	}

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "xxx",
			},
			Spec: corev1alpha1.ObjectSetSpec{
				Revision: 42,
			},
		},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)

	assert.True(t, res.IsZero(), "unexpected requeue")
	assert.Equal(t, objectSet.Spec.Revision, objectSet.Status.Revision)
}

func TestRevisionReconciler_SetSpecFromStatus(t *testing.T) {
	t.Parallel()

	testClient := testutil.NewClient()
	testClient.On("Update", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			os := args[1].(*corev1alpha1.ObjectSet)
			assert.NotZero(t, os.Spec.Revision)
			assert.Equal(t, os.Status.Revision, os.Spec.Revision)
		}).
		Return(nil)

	r := &revisionReconciler{
		scheme:       testScheme,
		newObjectSet: adapters.NewObjectSet,
		client:       testClient,
	}

	objectSet := &adapters.ObjectSetAdapter{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "xxx",
			},
			Status: corev1alpha1.ObjectSetStatus{
				Revision: 42,
			},
		},
	}

	ctx := context.Background()
	res, err := r.Reconcile(ctx, objectSet)
	require.NoError(t, err)

	assert.True(t, res.IsZero(), "unexpected requeue")
	assert.Equal(t, objectSet.GetStatusRevision(), objectSet.GetSpecRevision())

	testClient.AssertExpectations(t)
}
