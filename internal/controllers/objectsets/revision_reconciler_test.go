package objectsets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

var testScheme = runtime.NewScheme()

func init() {
	if err := corev1alpha1.AddToScheme(testScheme); err != nil {
		panic(err)
	}
}

func Test_revisionReconciler(t *testing.T) {
	t.Parallel()
	t.Run("defaults to revision 1", func(t *testing.T) {
		t.Parallel()
		testClient := testutil.NewClient()
		testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		r := &revisionReconciler{
			scheme:       testScheme,
			newObjectSet: newGenericObjectSet,
			client:       testClient,
		}

		objectSet := &GenericObjectSet{
			corev1alpha1.ObjectSet{},
		}

		ctx := context.Background()
		res, err := r.Reconcile(ctx, objectSet)
		require.NoError(t, err)

		assert.True(t, res.IsZero(), "unexpected requeue")
		assert.Equal(t, int64(1), objectSet.Status.Revision)
	})

	t.Run("sets revision based on previous", func(t *testing.T) {
		t.Parallel()

		testClient := testutil.NewClient()
		testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		r := &revisionReconciler{
			scheme:       testScheme,
			newObjectSet: newGenericObjectSet,
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

		objectSet := &GenericObjectSet{
			corev1alpha1.ObjectSet{
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
	})

	t.Run("waits on previous", func(t *testing.T) {
		t.Parallel()

		testClient := testutil.NewClient()
		testClient.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		r := &revisionReconciler{
			scheme:       testScheme,
			newObjectSet: newGenericObjectSet,
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

		objectSet := &GenericObjectSet{
			corev1alpha1.ObjectSet{
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
	})
}
