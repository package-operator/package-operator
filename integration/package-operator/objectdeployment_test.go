//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/controllers/objectdeployments"
)

func TestObjectDeployment_availability_and_hash_collision(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	testCases := []struct {
		deploymentRevision int
		probe              []corev1alpha1.ObjectSetProbe
		phases             []corev1alpha1.ObjectSetTemplatePhase
		// Objectset for the current deployment revision availability status.
		expectedRevisionAvailability metav1.ConditionStatus
		expectedObjectSetCount       int
		expectedHashCollisionCount   int
		expectedDeploymentConditions map[string]metav1.ConditionStatus
		expectedArchivedRevisions    []string
	}{
		{
			deploymentRevision: 1,
			probe:              hashCollisionTestProbe(),
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm1", "", map[string]string{"name": "cm1"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm2", "", map[string]string{"name": "cm2"}, t),
						},
					},
				},
			},
			expectedRevisionAvailability: metav1.ConditionTrue,
			expectedObjectSetCount:       1,
			expectedHashCollisionCount:   0,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable: metav1.ConditionTrue,
			},
		},
		{
			deploymentRevision: 2,
			probe:              hashCollisionTestProbe(),
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm1", "", map[string]string{"name": "cm2"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm2", "", map[string]string{"name": "fails"}, t),
						},
					},
				},
			},
			expectedRevisionAvailability: metav1.ConditionFalse,
			expectedObjectSetCount:       2,
			expectedHashCollisionCount:   1,
			// We handover cm1 and cm2 and then modify them to fail the probes.(Both in this revision and
			// the previous.)
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				// Now the previous revision which was earlier available also fails.
				// Thus making the deployment unavailable :(
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionFalse,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionTrue,
			},
		},
	}

	// Pre-Creating a ObjectSet that should conflict with Generation 2.
	existingConflictObjectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-objectdeployment-995cbf7d6",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			LifecycleState: corev1alpha1.ObjectSetLifecycleStatePaused,
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				AvailabilityProbes: hashCollisionTestProbe(),
				Phases:             testCases[1].phases,
			},
		},
	}
	require.NoError(t, Client.Create(ctx, existingConflictObjectSet))
	cleanupOnSuccess(ctx, t, existingConflictObjectSet)

	for _, testCase := range testCases {
		t.Logf("Running revision: %d \n", testCase.deploymentRevision)
		concernedDeployment := objectDeploymentTemplate(testCase.phases, testCase.probe, "test-objectdeployment", 10)
		currentInClusterDeployment := &corev1alpha1.ObjectDeployment{}
		err := Client.Get(ctx, client.ObjectKeyFromObject(concernedDeployment), currentInClusterDeployment)
		if errors.IsNotFound(err) {
			// Create the deployment
			require.NoError(t, Client.Create(ctx, concernedDeployment))
		} else {
			// Update the existing deployment
			concernedDeployment.ResourceVersion = currentInClusterDeployment.ResourceVersion
			require.NoError(t, Client.Update(ctx, concernedDeployment))
		}
		cleanupOnSuccess(ctx, t, concernedDeployment)

		// Assert that all the expected conditions are reported
		for expectedCond, expectedStatus := range testCase.expectedDeploymentConditions {
			requireCondition(ctx, t, concernedDeployment, expectedCond, expectedStatus)
		}

		// ObjectSet for the current deployment revision should be present
		currentObjectSet := &corev1alpha1.ObjectSet{}
		requireClientGet(ctx, t, ExpectedObjectSetName(concernedDeployment), concernedDeployment.Namespace, currentObjectSet)

		// Assert that the ObjectSet for the current revision has the expected availability status
		requireCondition(ctx, t, currentObjectSet, corev1alpha1.ObjectSetAvailable, testCase.expectedRevisionAvailability)

		// Assert that the ObjectSet reports the right TemplateHash
		require.Equal(t,
			concernedDeployment.Status.TemplateHash,
			currentObjectSet.GetAnnotations()[objectdeployments.ObjectSetHashAnnotation],
		)
		// Assert that the ObjectSet reports the right revision number
		require.Equal(t,
			concernedDeployment.Generation,
			currentObjectSet.Status.Revision) //nolint:staticcheck
		require.Equal(t,
			concernedDeployment.Generation,
			currentObjectSet.Spec.Revision)

		// Expect ObjectSet to be created
		// Expect concerned ObjectSet to be created
		currObjectSetList := listObjectSetRevisions(ctx, t, concernedDeployment)
		require.Len(t, currObjectSetList.Items, testCase.expectedObjectSetCount)

		// Assert that the expected revisions are archived (and others active)
		for _, currObjectSet := range currObjectSetList.Items {
			currObjectSetRevision := currObjectSet.Spec.Revision
			if slices.Contains(testCase.expectedArchivedRevisions, strconv.FormatInt(currObjectSetRevision, 10)) {
				requireCondition(ctx, t, currentObjectSet, corev1alpha1.ObjectSetArchived, metav1.ConditionTrue)
			} else {
				require.Equal(t, corev1alpha1.ObjectSetLifecycleStateActive, currObjectSet.Spec.LifecycleState)
			}
		}

		if testCase.expectedHashCollisionCount > 0 && concernedDeployment.Status.CollisionCount != nil {
			// Expect collision count to be the expected value
			require.Equal(t, *concernedDeployment.Status.CollisionCount, int32(testCase.expectedHashCollisionCount))
		}
	}
}

//nolint:maintidx
func TestObjectDeployment_ObjectSetArchival(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))
	testCases := []struct {
		revision string
		phases   []corev1alpha1.ObjectSetTemplatePhase
		probes   []corev1alpha1.ObjectSetProbe
		// Objectset for the current deployment revision availability status.
		expectedRevisionAvailability metav1.ConditionStatus
		expectedObjectSetCount       int
		expectedDeploymentConditions map[string]metav1.ConditionStatus
		expectedArchivedRevisions    []string
		expectedAvailableRevision    string
	}{
		{
			revision: "1",
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm1", "", map[string]string{"name": "probe-failure"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: deploymentTemplate("nginx-1", "nginx:1.14.1", t),
						},
					},
				},
			},
			probes:                       newArchivalTestProbes(),
			expectedRevisionAvailability: metav1.ConditionFalse,
			expectedObjectSetCount:       1,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable: metav1.ConditionFalse,
			},
			expectedAvailableRevision: "",
		},
		{
			revision: "2",
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm1", "", map[string]string{"name": "cm1"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: deploymentTemplate("nginx-1", "nginx:1.14.2", t),
						},
					},
				},
			},
			probes:                       newArchivalTestProbes(),
			expectedRevisionAvailability: metav1.ConditionTrue,
			expectedObjectSetCount:       2,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionFalse,
			},
			expectedArchivedRevisions: []string{"1"},
			expectedAvailableRevision: "2",
		},
		{
			revision: "3",
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm2", "", map[string]string{"name": "cm2"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: deploymentTemplate("nginx-2", "invalid-image", t),
						},
					},
				},
			},
			probes:                       newArchivalTestProbes(),
			expectedRevisionAvailability: metav1.ConditionFalse,
			expectedObjectSetCount:       3,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionTrue,
			},
			// Even though revision 2's actively reconciled objects are all diff
			// from this revision's objects, we cant archive revision 2 as its still available
			// (and this revision is not)
			expectedArchivedRevisions: []string{"1"},
			expectedAvailableRevision: "2",
		},
		{
			revision: "4",
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						// {
						// 	Object: deploymentTemplate("nginx-2", "nginx:1.14.2", t),
						// },
						{
							Object: secret("secret-1", t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm3", "", map[string]string{"name": "probe-failure"}, t),
						},
					},
				},
			},
			probes:                       newArchivalTestProbes(),
			expectedRevisionAvailability: metav1.ConditionFalse,
			expectedObjectSetCount:       4,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionTrue,
			},
			// Revision 3 can be archived as it doesnt actively reconcile any objects present in this revision
			expectedArchivedRevisions: []string{"1", "3"},
			expectedAvailableRevision: "2",
		},
		{
			revision: "5",
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm4", "", map[string]string{"name": "probe-failure"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: deploymentTemplate("nginx-3", "nginx:1.14.2", t),
						},
						{
							Object: secret("secret-1", t),
						},
					},
				},
			},
			probes:                       newArchivalTestProbes(),
			expectedRevisionAvailability: metav1.ConditionFalse,
			expectedObjectSetCount:       5,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionTrue,
			},
			// Revision 4 cant be archived as it actively reconciles secret which is in this revision
			expectedArchivedRevisions: []string{"1", "3"},
			expectedAvailableRevision: "2",
		},
		{
			revision: "6",
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm4", "", map[string]string{"name": "cm4"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: deploymentTemplate("nginx-3", "nginx:1.14.2", t),
						},
						{
							Object: secret("secret-1", t),
						},
					},
				},
			},
			probes:                       newArchivalTestProbes(),
			expectedRevisionAvailability: metav1.ConditionTrue,
			expectedObjectSetCount:       6,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionFalse,
			},
			// Revision 6 is available we can archive everything else
			expectedArchivedRevisions: []string{"1", "2", "3", "4", "5"},
			expectedAvailableRevision: "6",
		},
	}

	for _, testCase := range testCases {
		t.Logf("Running revision %s \n", testCase.revision)
		concernedDeployment := objectDeploymentTemplate(testCase.phases, testCase.probes, "test-objectdeployment-1", 10)
		currentInClusterDeployment := &corev1alpha1.ObjectDeployment{}
		err := Client.Get(ctx, client.ObjectKeyFromObject(concernedDeployment), currentInClusterDeployment)
		if errors.IsNotFound(err) {
			// Create the deployment
			require.NoError(t, Client.Create(ctx, concernedDeployment))
		} else {
			// Update the existing deployment
			concernedDeployment.ResourceVersion = currentInClusterDeployment.ResourceVersion
			require.NoError(t, Client.Update(ctx, concernedDeployment))
		}

		cleanupOnSuccess(ctx, t, concernedDeployment)
		// Assert that all the expected conditions are reported
		for expectedCond, expectedStatus := range testCase.expectedDeploymentConditions {
			requireCondition(ctx, t, concernedDeployment, expectedCond, expectedStatus)
		}

		// ObjectSet for the current deployment revision should be present
		currentObjectSet := &corev1alpha1.ObjectSet{}
		requireClientGet(ctx, t, ExpectedObjectSetName(concernedDeployment), concernedDeployment.Namespace, currentObjectSet)

		// Assert that the ObjectSet for the current revision has the expected availability status
		requireCondition(ctx, t, currentObjectSet, corev1alpha1.ObjectSetAvailable, testCase.expectedRevisionAvailability)

		// Assert that the ObjectSet reports the right TemplateHash
		require.Equal(t,
			concernedDeployment.Status.TemplateHash,
			currentObjectSet.GetAnnotations()[objectdeployments.ObjectSetHashAnnotation],
		)
		// Assert that the ObjectSet reports the right revision number
		require.Equal(t,
			concernedDeployment.Generation,
			currentObjectSet.Status.Revision) //nolint:staticcheck
		require.Equal(t,
			concernedDeployment.Generation,
			currentObjectSet.Spec.Revision)

		// Expect concerned ObjectSet to be created
		currObjectSetList := listObjectSetRevisions(ctx, t, concernedDeployment)
		require.Len(t, currObjectSetList.Items, testCase.expectedObjectSetCount)

		// Assert that the expected revisions are archived (and others active)
		for _, currObjectSet := range currObjectSetList.Items {
			currObjectSetRevision := currObjectSet.Spec.Revision
			if slices.Contains(testCase.expectedArchivedRevisions, strconv.FormatInt(currObjectSetRevision, 10)) {
				requireCondition(ctx, t, &currObjectSet, corev1alpha1.ObjectSetArchived, metav1.ConditionTrue)
			} else {
				require.Equal(t, corev1alpha1.ObjectSetLifecycleStateActive, currObjectSet.Spec.LifecycleState)
			}

			// Assert that expected revision is available
			if strconv.FormatInt(currObjectSetRevision, 10) == testCase.expectedAvailableRevision {
				availableCond := meta.FindStatusCondition(currObjectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
				require.NotNil(t, availableCond, "Available condition is expected to be reported")
				require.Equal(t, metav1.ConditionTrue, availableCond.Status)
			}
		}
	}
}

func TestObjectDeployment_Pause(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	testConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cm",
		},
		Data: map[string]string{"banana": "bread"},
	}

	objectDeployment := objectDeploymentTemplate([]corev1alpha1.ObjectSetTemplatePhase{
		{
			Name: "test-phase",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: cmTemplate(testConfigMap.Name, "", testConfigMap.Data, t),
				},
			},
		},
	}, nil, "test-od", 10)

	require.NoError(t, Client.Create(ctx, objectDeployment))
	cleanupOnSuccess(ctx, t, objectDeployment)

	// Assert that the ObjectDeployment reports available condition
	requireCondition(ctx, t, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue)

	// ObjectSet should be present
	objectSet := &corev1alpha1.ObjectSet{}
	requireClientGet(ctx, t, ExpectedObjectSetName(objectDeployment), objectDeployment.Namespace, objectSet)

	// Assert that the ObjectSet is available
	requireCondition(ctx, t, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue)

	// ConfigMap should be present
	cm := &corev1.ConfigMap{}
	requireClientGet(ctx, t, testConfigMap.Name, objectDeployment.Namespace, cm)
	assert.True(t, reflect.DeepEqual(testConfigMap.Data, cm.Data))

	// Pause ObjectDeployment Reconciliation
	objectDeployment.Spec.Paused = true
	require.NoError(t, Client.Update(ctx, objectDeployment))

	// Assert that the ObjectSet is paused
	requireCondition(ctx, t, objectSet, corev1alpha1.ObjectSetPaused, metav1.ConditionTrue)

	requireClientGet(ctx, t, objectSet.Name, objectSet.Namespace, objectSet)
	assert.Equal(t, corev1alpha1.ObjectSetLifecycleStatePaused, objectSet.Spec.LifecycleState)

	// Add a new config map to the paused ObjectDeployment
	requireClientGet(ctx, t, objectDeployment.Name, objectDeployment.Namespace, objectDeployment)
	newConfigMapName := "new-config-map"
	objectDeployment.Spec.Template.Spec.Phases[0].Objects = append(objectDeployment.Spec.Template.Spec.Phases[0].Objects,
		corev1alpha1.ObjectSetObject{
			Object: cmTemplate(newConfigMapName, "", nil, t),
		})
	require.NoError(t, Client.Update(ctx, objectDeployment))

	// The ObjectSet should not be archived
	require.ErrorIs(t,
		Waiter.WaitForCondition(ctx,
			objectSet,
			corev1alpha1.ObjectSetArchived,
			metav1.ConditionTrue,
		),
		context.DeadlineExceeded,
	)

	// No new revisions should be created
	objectSetList := listObjectSetRevisions(ctx, t, objectDeployment)
	assert.Len(t, objectSetList.Items, 1)

	// Unpause ObjectDeployment
	requireClientGet(ctx, t, objectDeployment.Name, objectDeployment.Namespace, objectDeployment)
	objectDeployment.Spec.Paused = false
	require.NoError(t, Client.Update(ctx, objectDeployment))

	// Wait for ObjectDeployment to be available
	requireCondition(ctx, t, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue)
	requireClientGet(ctx, t, objectDeployment.Name, objectDeployment.Namespace, objectDeployment)
	assert.True(t, meta.IsStatusConditionTrue(objectSet.Status.Conditions, corev1alpha1.ObjectDeploymentAvailable))
	assert.False(t, meta.IsStatusConditionTrue(objectSet.Status.Conditions, corev1alpha1.ObjectDeploymentProgressing))

	// A new revision should be created
	objectSetList = listObjectSetRevisions(ctx, t, objectDeployment)
	assert.Len(t, objectSetList.Items, 2)

	// New config map should be there
	requireClientGet(ctx, t, newConfigMapName, objectDeployment.Namespace, &testConfigMap)
}

func ExpectedObjectSetName(deployment *corev1alpha1.ObjectDeployment) string {
	return fmt.Sprintf("%s-%s", deployment.GetName(), deployment.Status.TemplateHash)
}

func secret(name string, t require.TestingT) unstructured.Unstructured {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"test.package-operator.run/test-1": "True"},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"hello": "world",
		},
	}
	GVK, err := apiutil.GVKForObject(&secret, Scheme)
	require.NoError(t, err)
	secret.SetGroupVersionKind(GVK)
	resObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&secret)
	require.NoError(t, err)
	return unstructured.Unstructured{Object: resObj}
}

func newArchivalTestProbes() []corev1alpha1.ObjectSetProbe {
	return []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Kind: "ConfigMap",
				},
			},
			Probes: []corev1alpha1.Probe{
				{
					FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
						FieldA: ".metadata.name",
						FieldB: ".data.name",
					},
				},
			},
		},
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Kind:  "Deployment",
					Group: "apps",
				},
			},
			Probes: []corev1alpha1.Probe{
				{
					FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
						FieldA: ".status.updatedReplicas",
						FieldB: ".status.replicas",
					},
				},
				{
					Condition: &corev1alpha1.ProbeConditionSpec{
						Type:   string(appsv1.DeploymentAvailable),
						Status: string(metav1.ConditionTrue),
					},
				},
			},
		},
	}
}

func listObjectSetRevisions(
	ctx context.Context, t *testing.T,
	objectDeployment *corev1alpha1.ObjectDeployment,
) *corev1alpha1.ObjectSetList {
	t.Helper()

	labelSelector := objectDeployment.Spec.Selector
	objectSetSelector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	require.NoError(t, err)

	objectSetList := &corev1alpha1.ObjectSetList{}
	err = Client.List(
		ctx, objectSetList,
		client.MatchingLabelsSelector{
			Selector: objectSetSelector,
		},
		client.InNamespace(objectDeployment.GetNamespace()),
	)
	require.NoError(t, err)

	return objectSetList
}

func requireClientGet(ctx context.Context, t *testing.T, name, namespace string, object client.Object) {
	t.Helper()

	require.NoError(t,
		Client.Get(ctx,
			client.ObjectKey{
				Name:      name,
				Namespace: namespace,
			},
			object,
		),
	)
}
