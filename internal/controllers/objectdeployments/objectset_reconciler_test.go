package objectdeployments

import (
	"context"
	"slices"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/adaptermocks"
)

const (
	testNamespace        = "test"
	objectDeploymentName = "test"
)

func Test_ObjectSetReconciler(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name                    string
		client                  *testutil.CtrlClient
		revisions               []corev1alpha1.ObjectSet
		deploymentGeneration    int64
		deploymentHash          string
		expectedCurrentRevision string
		expectedPrevRevisions   []string
		expectedConditions      map[string]metav1.ConditionStatus
	}{
		{
			name:   "latest revision available",
			client: testutil.NewClient(),
			revisions: []corev1alpha1.ObjectSet{
				newObjectSet("rev3", 3, "abcd", false, true, false),
				newObjectSet("rev1", 1, "xyz", false, true, false),
				newObjectSet("rev2", 2, "pqr", false, true, false),
				newObjectSet("rev4", 4, "abc", true, true, false),
			},
			deploymentGeneration:    4,
			deploymentHash:          "abc",
			expectedCurrentRevision: "rev4",
			expectedPrevRevisions:   []string{"rev1", "rev2", "rev3"},
			expectedConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionFalse,
			},
		},
		{
			name:   "no current revision",
			client: testutil.NewClient(),
			revisions: []corev1alpha1.ObjectSet{
				newObjectSet("rev3", 3, "abcd", false, true, false),
				newObjectSet("rev1", 1, "xyz", false, true, false),
				newObjectSet("rev2", 2, "pqr", false, true, false),
				newObjectSet("rev4", 4, "abc", true, true, false),
			},
			deploymentGeneration:    5,
			deploymentHash:          "hhh",
			expectedCurrentRevision: "",
			expectedPrevRevisions:   []string{"rev1", "rev2", "rev3", "rev4"},
			expectedConditions: map[string]metav1.ConditionStatus{
				// rev4 still available
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionTrue,
			},
		},
		{
			name:   "latest revision unavailable",
			client: testutil.NewClient(),
			revisions: []corev1alpha1.ObjectSet{
				newObjectSet("rev4", 4, "abc", false, false, false),
			},
			deploymentGeneration:    4,
			deploymentHash:          "abc",
			expectedCurrentRevision: "rev4",
			expectedPrevRevisions:   []string{},
			expectedConditions: map[string]metav1.ConditionStatus{
				// rev4 still available
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionFalse,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionTrue,
			},
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			client := testCase.client

			// Setup reconciler
			deploymentController := NewObjectDeploymentController(client, logr.Discard(), testScheme)
			mockedSubreconciler := &objectSetSubReconcilerMock{}

			mockedSubreconciler.On(
				"Reconcile",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(
				ctrl.Result{},
				nil,
			)
			r := objectSetReconciler{
				client:                      client,
				listObjectSetsForDeployment: deploymentController.listObjectSetsByRevision,
				reconcilers: []objectSetSubReconciler{
					mockedSubreconciler,
				},
			}

			existingConditions := []metav1.Condition{
				{
					Type:               corev1alpha1.ObjectDeploymentAvailable,
					Status:             metav1.ConditionFalse,
					Reason:             "ObjectSetUnready",
					Message:            "No ObjectSet is available.",
					ObservedGeneration: int64(1),
				},
			}

			objectDeploymentmock := newObjectDeploymentMock(
				testCase.deploymentGeneration,
				testCase.deploymentHash,
				&existingConditions,
			)

			// Return prepared revisions on client list
			revisions := testCase.revisions
			client.On("List",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Run(func(args mock.Arguments) {
				objectList := args.Get(1).(*corev1alpha1.ObjectSetList)
				objectList.Items = revisions
			}).Return(nil)

			// Invoke reconciler
			res, err := r.Reconcile(context.Background(), objectDeploymentmock)

			require.NoError(t, err, "unexpected error")
			require.True(t, res.IsZero(), "unexpected requeue")

			// Assert that the subreconcilers are called with the
			// correct args
			mockedSubreconciler.AssertCalled(
				t,
				"Reconcile",
				mock.Anything,
				mock.MatchedBy(func(item any) bool {
					if len(testCase.expectedCurrentRevision) == 0 {
						return item == nil
					}
					obj := item.(*adapters.ObjectSetAdapter)
					return obj.Name == testCase.expectedCurrentRevision
				}),
				mock.MatchedBy(func(obj any) bool {
					objs := obj.([]adapters.ObjectSetAccessor)
					if len(objs) != len(testCase.expectedPrevRevisions) {
						return false
					}
					for _, item := range objs {
						if !slices.Contains(testCase.expectedPrevRevisions, item.ClientObject().GetName()) {
							return false
						}
					}
					return true
				}),
				mock.Anything,
			)

			// Assert that the status is correctly set

			for expectedCondition, expectedStatus := range testCase.expectedConditions {
				cond := meta.FindStatusCondition(existingConditions, expectedCondition)
				require.NotNil(t, cond, "condition: "+expectedCondition+" should be reported")
				require.Equal(t, expectedStatus, cond.Status)
			}
		})
	}
}

func Test_ObjectDeploymentPause(t *testing.T) {
	t.Parallel()

	client := testutil.NewClient()

	// Setup reconciler
	deploymentController := NewObjectDeploymentController(client, logr.Discard(), testScheme)
	mockedSubreconciler := &objectSetSubReconcilerMock{}
	mockedSubreconciler.On(
		"Reconcile", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(ctrl.Result{}, nil)

	r := objectSetReconciler{
		client:                      client,
		listObjectSetsForDeployment: deploymentController.listObjectSetsByRevision,
		reconcilers: []objectSetSubReconciler{
			mockedSubreconciler,
		},
	}

	objectDeploymentmock := newObjectDeploymentMock(
		1,
		"test-hash-od",
		&[]metav1.Condition{{
			Type:               corev1alpha1.ObjectDeploymentAvailable,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: 1,
		}},
	)

	// Return prepared revisions on client list
	revisions := []corev1alpha1.ObjectSet{
		newObjectSet("rev1", 1, "test-hash-od", true, true, false),
	}
	client.On(
		"List", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Run(func(args mock.Arguments) {
		objectList := args.Get(1).(*corev1alpha1.ObjectSetList)
		objectList.Items = revisions
	}).Return(nil)

	// Pause ObjectDeployment
	objectDeploymentmock.On("GetSpecPaused").Unset()
	objectDeploymentmock.On("GetSpecPaused").Return(true)

	// Propagate reconciliation of objectSet to local variable
	client.On(
		"Update", mock.Anything, mock.Anything, mock.Anything,
	).Run(func(args mock.Arguments) {
		newObjectSet := args.Get(1).(*corev1alpha1.ObjectSet)
		revisions[0] = *newObjectSet
	}).Return(nil)
	res, err := r.Reconcile(context.Background(), objectDeploymentmock)
	require.NoError(t, err)
	assert.True(t, res.IsZero(), "unexpected requeue")

	// ObjectSet should be paused
	objectSets, err := r.listObjectSetsForDeployment(context.Background(), objectDeploymentmock)
	require.NoError(t, err)
	assert.Len(t, objectSets, 1)
	assert.True(t, objectSets[0].IsSpecPaused())

	// Set objectSet status to paused
	revisions[0].Status.Conditions = append(revisions[0].Status.Conditions, metav1.Condition{
		Type:               corev1alpha1.ObjectSetPaused,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: 1,
	})

	res, err = r.Reconcile(context.Background(), objectDeploymentmock)
	require.NoError(t, err)
	assert.True(t, res.IsZero(), "unexpected requeue")

	// ObjectDeployment status should reflect objectSet's paused status
	objectDeploymentmock.AssertCalled(t, "SetStatusConditions", []metav1.Condition{
		newPausedCondition(metav1.ConditionTrue, pausedReasonPaused, "Latest revision is paused: ", 1),
	})

	// Unpause objectDeployment
	objectDeploymentmock.On("GetSpecPaused").Unset()
	objectDeploymentmock.On("GetSpecPaused").Return(false)

	res, err = r.Reconcile(context.Background(), objectDeploymentmock)
	require.NoError(t, err)
	assert.True(t, res.IsZero(), "unexpected requeue")

	// ObjectSet should be un-paused
	objectSets, err = r.listObjectSetsForDeployment(context.Background(), objectDeploymentmock)
	require.NoError(t, err)
	assert.Len(t, objectSets, 1)
	assert.False(t, objectSets[0].IsSpecPaused())

	// Set objectSet status to un-paused
	meta.FindStatusCondition(revisions[0].Status.Conditions, corev1alpha1.ObjectSetPaused).Status = metav1.ConditionFalse

	res, err = r.Reconcile(context.Background(), objectDeploymentmock)
	require.NoError(t, err)
	assert.True(t, res.IsZero(), "unexpected requeue")

	// ObjectDeployment status should reflect objectSet's un-paused status
	objectDeploymentmock.AssertCalled(t, "RemoveStatusConditions", []string{corev1alpha1.ObjectDeploymentPaused})
}

func newObjectDeploymentMock(
	generation int64,
	templateHash string,
	initialConditions *[]metav1.Condition,
) *adaptermocks.ObjectDeploymentMock {
	res := &adaptermocks.ObjectDeploymentMock{}
	obj := &corev1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       objectDeploymentName,
			Namespace:  testNamespace,
			Generation: generation,
		},
	}
	GVK, err := apiutil.GVKForObject(obj, testScheme)
	if err != nil {
		panic(err)
	}
	obj.SetGroupVersionKind(GVK)
	res.On("ClientObject").Return(
		obj,
	)
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"match": "all",
		},
	}
	res.On("SetStatusRevision", mock.Anything).Return()
	res.On("GetSpecSelector").Return(labelSelector)
	res.On("GetGeneration").Return(generation)
	res.On("GetStatusTemplateHash").Return(templateHash)
	res.On("GetStatusConditions").Return(initialConditions)
	res.On("GetName").Return(objectDeploymentName)
	res.On("SetStatusConditions", mock.Anything).Run(func(args mock.Arguments) {
		conds := args.Get(0).([]metav1.Condition)

		for _, c := range conds {
			c.ObservedGeneration = generation

			meta.SetStatusCondition(initialConditions, c)
		}
	}).Return()
	res.On("SetStatusCollisionCount", mock.Anything).Return()
	res.On("GetStatusCollisionCount").Return(nil)
	res.On("GetNamespace").Return(testNamespace)
	res.On("GetAnnotations").Return(map[string]string{})
	res.On("GetSpecObjectSetTemplate").Return(
		corev1alpha1.ObjectSetTemplate{Spec: corev1alpha1.ObjectSetTemplateSpec{
			Phases: []corev1alpha1.ObjectSetTemplatePhase{{}},
		}},
	)
	res.On("SetStatusControllerOf", mock.Anything).Return()
	res.On("GetStatusControllerOf").Return(nil)
	res.On("RemoveStatusConditions", mock.Anything).Return()
	res.On("GetSpecPaused").Return(false)
	res.On("SetSpecPaused", mock.Anything).Return()
	return res
}

func newObjectSet(
	name string,
	deploymentRevision int64, hash string,
	available bool, successful bool, archived bool,
) corev1alpha1.ObjectSet {
	obj := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Annotations: map[string]string{
				ObjectSetHashAnnotation: hash,
			},
		},
		Spec: corev1alpha1.ObjectSetSpec{
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{{}},
			},
			Revision: deploymentRevision,
		},
		Status: corev1alpha1.ObjectSetStatus{
			Revision: deploymentRevision,
		},
	}

	if available {
		obj.Status.Conditions = append(obj.Status.Conditions,
			metav1.Condition{
				Type:   corev1alpha1.ObjectSetAvailable,
				Status: metav1.ConditionTrue,
			},
		)
	} else {
		obj.Status.Conditions = append(obj.Status.Conditions,
			metav1.Condition{
				Type:   corev1alpha1.ObjectSetAvailable,
				Status: metav1.ConditionFalse,
			},
		)
	}

	if successful {
		obj.Status.Conditions = append(obj.Status.Conditions,
			metav1.Condition{
				Type:   corev1alpha1.ObjectSetSucceeded,
				Status: metav1.ConditionTrue,
			},
		)
	} else {
		obj.Status.Conditions = append(obj.Status.Conditions,
			metav1.Condition{
				Type:   corev1alpha1.ObjectSetSucceeded,
				Status: metav1.ConditionFalse,
			},
		)
	}

	if archived {
		obj.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived
		obj.Status.Conditions = append(obj.Status.Conditions,
			metav1.Condition{
				Type:   corev1alpha1.ObjectSetArchived,
				Status: metav1.ConditionTrue,
			},
		)
	}

	return *obj
}
