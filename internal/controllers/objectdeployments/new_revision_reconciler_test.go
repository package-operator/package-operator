package objectdeployments

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func Test_newRevisionReconciler_delaysObjectSetCreation(
	t *testing.T,
) {
	log := testr.New(t)
	ctx := logr.NewContext(context.Background(), log)
	clientMock := testutil.NewClient()
	deploymentController := NewObjectDeploymentController(clientMock, log, testScheme)
	r := newRevisionReconciler{
		client:       clientMock,
		newObjectSet: deploymentController.newObjectSet,
		scheme:       testScheme,
	}

	objectDeploymentMock := &genericObjectDeploymentMock{}
	objectDeploymentMock.
		On("GetObjectSetTemplate").
		Return(corev1alpha1.ObjectSetTemplate{})

	res, err := r.Reconcile(ctx, nil, nil, objectDeploymentMock)
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertNotCalled(
		t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func Test_newRevisionReconciler_createsObjectSet(t *testing.T) {
	testCases := []struct {
		name                       string
		client                     *testutil.CtrlClient
		prevRevisions              []corev1alpha1.ObjectSet
		deploymentGeneration       int64
		deploymentHash             string
		conflict                   bool
		conflictObject             corev1alpha1.ObjectSet
		expectedHashCollisionCount int
	}{
		{
			name:   "success",
			client: testutil.NewClient(),
			prevRevisions: []corev1alpha1.ObjectSet{
				makeObjectSet("rev3", "test", 3, "abcd", false, true),
				makeObjectSet("rev1", "test", 1, "xyz", false, true),
				makeObjectSet("rev2", "test", 2, "pqr", false, true),
				makeObjectSet("rev4", "test", 4, "abc", true, true),
			},
			deploymentGeneration:       5,
			deploymentHash:             "test1",
			conflict:                   false,
			expectedHashCollisionCount: 0,
		},
		{
			name:   "hash collision",
			client: testutil.NewClient(),
			prevRevisions: []corev1alpha1.ObjectSet{
				makeObjectSet("rev3", "test", 3, "abcd", false, false),
				makeObjectSet("rev1", "test", 1, "xyz", true, true),
				makeObjectSet("rev2", "test", 2, "pqr", false, false),
				makeObjectSet("rev4", "test", 4, "abc", false, false),
			},
			deploymentGeneration:       5,
			deploymentHash:             "xyz",
			conflict:                   true,
			conflictObject:             makeObjectSet("test-xyz", "test", 1, "xyz", true, true),
			expectedHashCollisionCount: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			log := testr.New(t)
			ctx := logr.NewContext(context.Background(), log)
			clientMock := testCase.client
			// Setup reconciler
			deploymentController := NewObjectDeploymentController(testCase.client, log, testScheme)
			r := newRevisionReconciler{
				client:       clientMock,
				newObjectSet: deploymentController.newObjectSet,
				scheme:       testScheme,
			}

			objectDeploymentMock := makeObjectDeploymentMock(
				"test",
				"test",
				testCase.deploymentGeneration,
				testCase.deploymentHash,
				nil,
			)

			// If conflict object is present
			// make the client return an AlreadyExists error
			if testCase.conflict {
				clientMock.On("Create",
					mock.Anything,
					mock.Anything,
					[]ctrlclient.CreateOption(nil),
				).Return(errors.NewAlreadyExists(schema.GroupResource{}, testCase.conflictObject.Name))
				clientMock.On("Get",
					mock.Anything,
					ctrlclient.ObjectKey{
						Name:      testCase.conflictObject.Name,
						Namespace: testCase.conflictObject.Namespace,
					},
					mock.Anything,
					mock.Anything,
				).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1alpha1.ObjectSet)
					*obj = testCase.conflictObject
				}).
					Return(nil)
			} else {
				clientMock.On("Create",
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
			res, err := r.Reconcile(ctx, nil, revisions, objectDeploymentMock)
			require.NoError(t, err, "unexpected error")
			require.True(t, res.IsZero(), "unexpected requeue")

			// assert hash collisions
			if testCase.expectedHashCollisionCount > 0 {
				expectedCollison := int32(testCase.expectedHashCollisionCount)
				objectDeploymentMock.AssertCalled(t, "SetStatusCollisionCount", &expectedCollison)
			} else {
				objectDeploymentMock.AssertNotCalled(t, "SetStatusCollisionCount", mock.AnythingOfType("*int32"))
			}

			// Assert correct new revision is created
			clientMock.AssertCalled(
				t,
				"Create",
				mock.Anything,
				mock.MatchedBy(func(item interface{}) bool {
					obj := item.(*corev1alpha1.ObjectSet)
					requireObject(t, obj, testCase.deploymentHash, testCase.prevRevisions)

					return true
				}),
				[]ctrlclient.CreateOption(nil),
			)
		})
	}
}

func requireObject(t *testing.T,
	obj *corev1alpha1.ObjectSet,
	expectedHash string,
	prevs []corev1alpha1.ObjectSet,
) {
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
}
