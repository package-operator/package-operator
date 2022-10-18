package objectdeployments

import (
	"context"
	"fmt"
	"testing"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func Test_new_revision_reconciler(t *testing.T) {
	t.Run("Creates a new objectset with the correct attributes or handles hash collision", func(t *testing.T) {
		testCases := []struct {
			client                     *testutil.CtrlClient
			prevRevisions              []corev1alpha1.ObjectSet
			deploymentGeneration       int
			deploymentHash             string
			conflict                   bool
			conflictObject             corev1alpha1.ObjectSet
			expectedHashCollisionCount int
		}{
			{
				client: testutil.NewClient(),
				prevRevisions: []corev1alpha1.ObjectSet{
					makeObjectSet("rev3", 3, "abcd", false),
					makeObjectSet("rev1", 1, "xyz", false),
					makeObjectSet("rev2", 2, "pqr", false),
					makeObjectSet("rev4", 4, "abc", true),
				},
				deploymentGeneration:       5,
				deploymentHash:             "test1",
				conflict:                   false,
				expectedHashCollisionCount: 0,
			},
			// hash collision
			{
				client: testutil.NewClient(),
				prevRevisions: []corev1alpha1.ObjectSet{
					makeObjectSet("rev3", 3, "abcd", false),
					makeObjectSet("rev1", 1, "xyz", true),
					makeObjectSet("rev2", 2, "pqr", false),
					makeObjectSet("rev4", 4, "abc", false),
				},
				deploymentGeneration:       5,
				deploymentHash:             "xyz",
				conflict:                   true,
				conflictObject:             makeObjectSet("rev1", 1, "xyz", true),
				expectedHashCollisionCount: 1,
			},
			// Object already present, but sanity check kicks in
			// so no collision count not incremented.
			{
				client: testutil.NewClient(),
				prevRevisions: []corev1alpha1.ObjectSet{
					makeObjectSet("rev3", 3, "abcd", false),
					makeObjectSet("rev1", 1, "xyz", true),
					makeObjectSet("rev2", 2, "pqr", false),
					makeObjectSet("rev4", 4, "abc", false),
				},
				deploymentGeneration:       4,
				deploymentHash:             "abc",
				conflict:                   true,
				conflictObject:             makeObjectSet("rev4", 4, "abc", true),
				expectedHashCollisionCount: 0,
			},
		}

		for _, testCase := range testCases {
			client := testCase.client
			// Setup reconciler
			deploymentController := NewObjectDeploymentController(testCase.client, logr.Discard(), testScheme)
			r := newRevisionReconciler{
				client:       client,
				newObjectSet: deploymentController.newObjectSet,
				scheme:       testScheme,
			}

			objectDeploymentmock := makeObjectDeploymentMock(
				"test",
				"test",
				testCase.deploymentGeneration,
				testCase.deploymentHash,
				nil,
			)

			// If conflict object is present
			// make the client return an AlreadyExists error
			if testCase.conflict {
				client.On("Create",
					mock.Anything,
					mock.Anything,
					[]ctrlclient.CreateOption(nil),
				).Return(errors.NewAlreadyExists(schema.GroupResource{}, testCase.conflictObject.Name))

				client.On("Get",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Run(func(args mock.Arguments) {
					arg := args.Get(2)
					obj := arg.(*corev1alpha1.ObjectSet)
					*obj = testCase.conflictObject
				}).Return(nil)
			} else {
				client.On("Create",
					mock.Anything,
					mock.Anything,
					[]ctrlclient.CreateOption(nil),
				).Return(nil)
			}

			revisions := make([]genericObjectSet, len(testCase.prevRevisions))

			for i := range testCase.prevRevisions {
				revisions[i] = &GenericObjectSet{
					testCase.prevRevisions[i],
				}
			}

			// Invoke reconciler
			res, err := r.Reconcile(context.Background(), nil, revisions, objectDeploymentmock)
			require.NoError(t, err, "unexpected error")
			require.True(t, res.IsZero(), "unexpected requeue")

			// assert hash collisions
			if testCase.expectedHashCollisionCount > 0 {
				expectedCollison := int32(testCase.expectedHashCollisionCount)
				objectDeploymentmock.AssertCalled(t, "SetStatusCollisionCount", &expectedCollison)
			} else {
				objectDeploymentmock.AssertNotCalled(t, "SetStatusCollisionCount")
			}

			// Assert correct new revision is created
			client.AssertCalled(
				t,
				"Create",
				mock.Anything,
				mock.MatchedBy(func(item interface{}) bool {
					obj := item.(*corev1alpha1.ObjectSet)
					return assertObject(t,
						obj,
						testCase.deploymentHash,
						fmt.Sprint(testCase.deploymentGeneration),
						testCase.prevRevisions,
					)
				}),
				[]ctrlclient.CreateOption(nil),
			)
		}
	})

}

func assertObject(t *testing.T,
	obj *corev1alpha1.ObjectSet,
	expectedHash string,
	expectedRevision string,
	prevs []corev1alpha1.ObjectSet) bool {
	t.Helper()
	hash, ok1 := obj.Annotations[ObjectSetHashAnnotation]
	require.True(t, ok1)
	require.Equal(t, hash, expectedHash)
	require.True(t, len(prevs) == len(obj.Spec.Previous))

	objprevs := make([]string, len(obj.Spec.Previous))
	for i, prev := range obj.Spec.Previous {
		objprevs[i] = prev.Name
	}
	for _, prev := range prevs {
		require.Contains(t, objprevs, prev.Name)
	}
	return true
}
