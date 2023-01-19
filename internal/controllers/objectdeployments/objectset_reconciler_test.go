package objectdeployments

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

func Test_ObjectSetReconciler(t *testing.T) {
	testCases := []struct {
		client                  *testutil.CtrlClient
		revisions               []corev1alpha1.ObjectSet
		deploymentGeneration    int
		deploymentHash          string
		expectedCurrentRevision string
		expectedPrevRevisions   []string
		expectedConditions      map[string]metav1.ConditionStatus
	}{
		{
			client: testutil.NewClient(),
			revisions: []corev1alpha1.ObjectSet{
				makeObjectSet("rev3", "test", 3, "abcd", false),
				makeObjectSet("rev1", "test", 1, "xyz", false),
				makeObjectSet("rev2", "test", 2, "pqr", false),
				makeObjectSet("rev4", "test", 4, "abc", true),
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
		// No current revision
		{
			client: testutil.NewClient(),
			revisions: []corev1alpha1.ObjectSet{
				makeObjectSet("rev3", "test", 3, "abcd", false),
				makeObjectSet("rev1", "test", 1, "xyz", false),
				makeObjectSet("rev2", "test", 2, "pqr", false),
				makeObjectSet("rev4", "test", 4, "abc", true),
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
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("%d:Currently calls the subsreconcilers and sets the correct condition", i), func(t *testing.T) {
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

			objectDeploymentmock := makeObjectDeploymentMock(
				"test",
				"test",
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
				mock.MatchedBy(func(item interface{}) bool {
					if len(testCase.expectedCurrentRevision) == 0 {
						return item == nil
					}
					obj := item.(*GenericObjectSet)
					return obj.Name == testCase.expectedCurrentRevision
				}),
				mock.MatchedBy(func(obj interface{}) bool {
					objs := obj.([]genericObjectSet)
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
				require.NotNil(t, cond)
				require.True(t, cond.Status == expectedStatus)
			}
		})
	}
}

func makeObjectDeploymentMock(name string, namespace string,
	generation int,
	templateHash string,
	initialConditions *[]metav1.Condition) *genericObjectDeploymentMock {
	res := &genericObjectDeploymentMock{}
	obj := &corev1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: int64(generation),
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
	res.On("GetSelector").Return(labelSelector)
	res.On("GetGeneration").Return(int64(generation))
	res.On("GetStatusTemplateHash").Return(templateHash)
	res.On("GetConditions").Return(initialConditions)
	res.On("GetName").Return(name)
	res.On("SetStatusCollisionCount", mock.Anything).Return()
	res.On("GetStatusCollisionCount").Return(nil)
	res.On("GetNamespace").Return(namespace)
	res.On("GetAnnotations").Return(map[string]string{})
	res.On("GetObjectSetTemplate").Return(
		corev1alpha1.ObjectSetTemplate{Spec: corev1alpha1.ObjectSetTemplateSpec{
			Phases: []corev1alpha1.ObjectSetTemplatePhase{{}},
		}},
	)
	return res
}

func makeObjectSet(name, namespace string, deploymentRevision int64, hash string, available bool) corev1alpha1.ObjectSet {
	obj := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				ObjectSetHashAnnotation: hash,
			},
		},
		Status: corev1alpha1.ObjectSetStatus{
			Revision: deploymentRevision,
		},
	}
	if available {
		obj.Status.Conditions = []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetAvailable,
				Status: metav1.ConditionTrue,
			},
		}
	} else {
		obj.Status.Conditions = []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetAvailable,
				Status: metav1.ConditionFalse,
			},
		}
	}
	return *obj
}
