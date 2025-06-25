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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/adaptermocks"
)

func Test_newRevisionReconciler_delaysObjectSetCreation(t *testing.T) {
	t.Parallel()
	log := testr.New(t)
	ctx := logr.NewContext(context.Background(), log)
	clientMock := testutil.NewClient()
	deploymentController := NewObjectDeploymentController(clientMock, log, testScheme)
	r := newRevisionReconciler{
		client:       clientMock,
		newObjectSet: deploymentController.newObjectSet,
		scheme:       testScheme,
	}

	objectDeploymentMock := &adaptermocks.ObjectDeploymentMock{}
	objectDeploymentMock.
		On("GetSpecObjectSetTemplate").
		Return(corev1alpha1.ObjectSetTemplate{})

	res, err := r.Reconcile(ctx, nil, nil, objectDeploymentMock)
	require.NoError(t, err)
	assert.True(t, res.IsZero())

	clientMock.AssertNotCalled(
		t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func Test_newRevisionReconciler_createsObjectSet(t *testing.T) {
	t.Parallel()

	hashCollisionOS := newObjectSet("test-xyz", 1, "xyz", true, true, false)
	hashCollisionOS.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
		{}, {},
	}

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
				newObjectSet("rev3", 3, "abcd", false, true, false),
				newObjectSet("rev1", 1, "xyz", false, true, false),
				newObjectSet("rev2", 2, "pqr", false, true, false),
				newObjectSet("rev4", 4, "abc", true, true, false),
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
				newObjectSet("rev3", 3, "abcd", false, false, false),
				newObjectSet("rev1", 1, "xyz", true, true, false),
				newObjectSet("rev2", 2, "pqr", false, false, false),
				newObjectSet("rev4", 4, "abc", false, false, false),
			},
			deploymentGeneration:       5,
			deploymentHash:             "xyz",
			conflict:                   true,
			conflictObject:             hashCollisionOS,
			expectedHashCollisionCount: 1,
		},
		{
			name:   "hash collision - slow cache",
			client: testutil.NewClient(),
			prevRevisions: []corev1alpha1.ObjectSet{
				newObjectSet("rev3", 3, "abcd", false, false, false),
				newObjectSet("rev1", 1, "xyz", true, true, false),
				newObjectSet("rev2", 2, "pqr", false, false, false),
				newObjectSet("rev4", 4, "abc", false, false, false),
			},
			deploymentGeneration:       5,
			deploymentHash:             "xyz",
			conflict:                   true,
			conflictObject:             newObjectSet("test-xyz", 4, "xyz", true, true, false),
			expectedHashCollisionCount: 0,
		},
		{
			name:   "hash collision archived",
			client: testutil.NewClient(),
			prevRevisions: []corev1alpha1.ObjectSet{
				newObjectSet("rev3", 3, "abcd", false, false, false),
				newObjectSet("rev1", 1, "xyz", true, true, true),
				newObjectSet("rev2", 2, "pqr", false, false, false),
				newObjectSet("rev4", 4, "abc", false, false, false),
			},
			deploymentGeneration:       5,
			deploymentHash:             "xyz",
			conflict:                   true,
			conflictObject:             newObjectSet("test-xyz", 1, "xyz", true, true, true),
			expectedHashCollisionCount: 1,
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
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

			objectDeployment := adapters.NewObjectDeployment(testScheme)
			objectDeployment.ClientObject().SetName(objectDeploymentName)
			objectDeployment.ClientObject().SetNamespace(testNamespace)
			objectDeployment.ClientObject().SetGeneration(testCase.deploymentGeneration)
			objectDeployment.SetSpecTemplateSpec(corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{{}},
			})
			objectDeployment.SetStatusTemplateHash(testCase.deploymentHash)

			// If conflict object is present
			// make the client return an AlreadyExists error
			if testCase.conflict {
				if err := controllerutil.SetControllerReference(
					objectDeployment.ClientObject(), &testCase.conflictObject, testScheme); err != nil {
					require.NoError(t, err)
				}

				clientMock.On("Create",
					mock.Anything,
					mock.Anything,
					[]client.CreateOption(nil),
				).Return(errors.NewAlreadyExists(schema.GroupResource{}, testCase.conflictObject.Name))
				clientMock.On("Get",
					mock.Anything,
					client.ObjectKey{
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
					[]client.CreateOption(nil),
				).Return(nil)
			}

			revisions := make([]adapters.ObjectSetAccessor, len(testCase.prevRevisions))

			for i := range testCase.prevRevisions {
				obj := &testCase.prevRevisions[i]
				if err := controllerutil.SetControllerReference(
					objectDeployment.ClientObject(), obj, testScheme); err != nil {
					require.NoError(t, err)
				}

				revisions[i] = &adapters.ObjectSetAdapter{
					ObjectSet: testCase.prevRevisions[i],
				}
			}

			// Invoke reconciler
			res, err := r.Reconcile(ctx, nil, revisions, objectDeployment)
			require.NoError(t, err, "unexpected error")
			require.True(t, res.IsZero(), "unexpected requeue")

			// assert hash collisions
			if testCase.expectedHashCollisionCount > 0 {
				expectedCollison := int32(testCase.expectedHashCollisionCount)

				actualCount := objectDeployment.GetStatusCollisionCount()
				if assert.NotNil(t, actualCount) {
					assert.Equal(t, expectedCollison, *actualCount)
				}
			} else {
				assert.Nil(t, objectDeployment.GetStatusCollisionCount())
			}

			// Assert correct new revision is created
			clientMock.AssertCalled(
				t,
				"Create",
				mock.Anything,
				mock.MatchedBy(func(item any) bool {
					obj := item.(*corev1alpha1.ObjectSet)
					requireObject(t, obj, testCase.deploymentHash, testCase.prevRevisions)

					return true
				}),
				[]client.CreateOption(nil),
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
	require.Equal(t, expectedHash, hash)
	require.Len(t, obj.Spec.Previous, len(prevs))

	objprevs := make([]string, len(obj.Spec.Previous))
	for i, prev := range obj.Spec.Previous {
		objprevs[i] = prev.Name
	}
	for _, prev := range prevs {
		require.Contains(t, objprevs, prev.Name)
	}
}
