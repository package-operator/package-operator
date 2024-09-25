//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
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
			probe:              hashCollisionTestProbe(".metadata.name", ".data.name"),
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm1", map[string]string{"name": "cm1"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm2", map[string]string{"name": "cm2"}, t),
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
			probe:              hashCollisionTestProbe(".metadata.name", ".data.name"),
			phases: []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm1", map[string]string{"name": "cm2"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate("cm2", map[string]string{"name": "fails"}, t),
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
				AvailabilityProbes: hashCollisionTestProbe(".metadata.name", ".data.name"),
				Phases:             testCases[1].phases,
			},
		},
	}
	require.NoError(t, Client.Create(ctx, existingConflictObjectSet))
	cleanupOnSuccess(ctx, t, existingConflictObjectSet)

	for _, testCase := range testCases {
		t.Logf("Running revision: %d \n", testCase.deploymentRevision)
		concernedDeployment := objectDeploymentTemplate(testCase.phases, testCase.probe, "test-objectdeployment")
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
			require.NoError(t,
				Waiter.WaitForCondition(ctx,
					concernedDeployment,
					expectedCond,
					expectedStatus,
				),
			)
			cond := meta.FindStatusCondition(concernedDeployment.Status.Conditions, expectedCond)
			require.Equal(t, expectedStatus, cond.Status)
		}

		// ObjectSet for the current deployment revision should be present
		currentObjectSet := &corev1alpha1.ObjectSet{}
		require.NoError(t,
			Client.Get(ctx,
				client.ObjectKey{
					Name:      ExpectedObjectSetName(concernedDeployment),
					Namespace: concernedDeployment.Namespace,
				},
				currentObjectSet,
			),
		)

		// Assert that the ObjectSet for the current revision has the expected availability status
		require.NoError(t,
			Waiter.WaitForCondition(ctx,
				currentObjectSet,
				corev1alpha1.ObjectSetAvailable,
				testCase.expectedRevisionAvailability,
			),
		)
		availableCond := meta.FindStatusCondition(currentObjectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
		require.NotNil(t, availableCond, "Available condition is expected to be reported")
		require.Equal(t, testCase.expectedRevisionAvailability, availableCond.Status)

		// Assert that the ObjectSet reports the right TemplateHash
		require.Equal(t,
			concernedDeployment.Status.TemplateHash,
			currentObjectSet.GetAnnotations()[objectdeployments.ObjectSetHashAnnotation],
		)
		// Assert that the ObjectSet reports the right revision number
		require.Equal(t,
			concernedDeployment.Generation,
			currentObjectSet.Status.Revision)

		// Expect ObjectSet to be created
		// Expect concerned ObjectSet to be created
		labelSelector := concernedDeployment.Spec.Selector
		objectSetSelector, err := metav1.LabelSelectorAsSelector(&labelSelector)
		require.NoError(t, err)
		currObjectSetList := &corev1alpha1.ObjectSetList{}
		err = Client.List(
			ctx, currObjectSetList,
			client.MatchingLabelsSelector{
				Selector: objectSetSelector,
			},
			client.InNamespace(concernedDeployment.GetNamespace()),
		)
		require.NoError(t, err)

		require.Len(t, currObjectSetList.Items, testCase.expectedObjectSetCount)

		// Assert that the expected revisions are archived (and others active)
		for _, currObjectSet := range currObjectSetList.Items {
			currObjectSetRevision := currObjectSet.Status.Revision
			if slices.Contains(testCase.expectedArchivedRevisions, strconv.FormatInt(currObjectSetRevision, 10)) {
				require.NoError(t,
					Waiter.WaitForCondition(ctx,
						&currObjectSet,
						corev1alpha1.ObjectSetArchived,
						metav1.ConditionTrue,
					),
				)
				archivedCond := meta.FindStatusCondition(currObjectSet.Status.Conditions, corev1alpha1.ObjectSetArchived)
				require.NotNil(t, archivedCond, "Archived condition is expected to be reported")
				require.Equal(t, metav1.ConditionTrue, archivedCond.Status)
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
							Object: cmTemplate("cm1", map[string]string{"name": "probe-failure"}, t),
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
			probes:                       archivalTestprobesTemplate(".metadata.name", ".data.name"),
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
							Object: cmTemplate("cm1", map[string]string{"name": "cm1"}, t),
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
			probes:                       archivalTestprobesTemplate(".metadata.name", ".data.name"),
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
							Object: cmTemplate("cm2", map[string]string{"name": "cm2"}, t),
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
			probes:                       archivalTestprobesTemplate(".metadata.name", ".data.name"),
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
							Object: cmTemplate("cm3", map[string]string{"name": "probe-failure"}, t),
						},
					},
				},
			},
			probes:                       archivalTestprobesTemplate(".metadata.name", ".data.name"),
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
							Object: cmTemplate("cm4", map[string]string{"name": "probe-failure"}, t),
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
			probes:                       archivalTestprobesTemplate(".metadata.name", ".data.name"),
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
							Object: cmTemplate("cm4", map[string]string{"name": "cm4"}, t),
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
			probes:                       archivalTestprobesTemplate(".metadata.name", ".data.name"),
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
		concernedDeployment := objectDeploymentTemplate(testCase.phases, testCase.probes, "test-objectdeployment-1")
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
			require.NoError(t,
				Waiter.WaitForCondition(ctx,
					concernedDeployment,
					expectedCond,
					expectedStatus,
				),
			)
			cond := meta.FindStatusCondition(concernedDeployment.Status.Conditions, expectedCond)
			require.Equal(t, expectedStatus, cond.Status)
		}

		// ObjectSet for the current deployment revision should be present
		currentObjectSet := &corev1alpha1.ObjectSet{}
		require.NoError(t,
			Client.Get(ctx,
				client.ObjectKey{
					Name:      ExpectedObjectSetName(concernedDeployment),
					Namespace: concernedDeployment.Namespace,
				},
				currentObjectSet,
			),
		)

		// Assert that the ObjectSet for the current revision has the expected availability status
		require.NoError(t,
			Waiter.WaitForCondition(ctx,
				currentObjectSet,
				corev1alpha1.ObjectSetAvailable,
				testCase.expectedRevisionAvailability,
			),
		)
		availableCond := meta.FindStatusCondition(currentObjectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
		require.NotNil(t, availableCond, "Available condition is expected to be reported")
		require.Equal(t, testCase.expectedRevisionAvailability, availableCond.Status)

		// Assert that the ObjectSet reports the right TemplateHash
		require.Equal(t,
			concernedDeployment.Status.TemplateHash,
			currentObjectSet.GetAnnotations()[objectdeployments.ObjectSetHashAnnotation],
		)
		// Assert that the ObjectSet reports the right revision number
		require.Equal(t,
			concernedDeployment.Generation,
			currentObjectSet.Status.Revision)

		// Expect concerned ObjectSet to be created
		labelSelector := concernedDeployment.Spec.Selector
		objectSetSelector, err := metav1.LabelSelectorAsSelector(&labelSelector)
		require.NoError(t, err)
		currObjectSetList := &corev1alpha1.ObjectSetList{}
		err = Client.List(
			ctx, currObjectSetList,
			client.MatchingLabelsSelector{
				Selector: objectSetSelector,
			},
			client.InNamespace(concernedDeployment.GetNamespace()),
		)
		require.NoError(t, err)

		require.Len(t, currObjectSetList.Items, testCase.expectedObjectSetCount)

		// Assert that the expected revisions are archived (and others active)
		for _, currObjectSet := range currObjectSetList.Items {
			currObjectSetRevision := currObjectSet.Status.Revision
			if slices.Contains(testCase.expectedArchivedRevisions, strconv.FormatInt(currObjectSetRevision, 10)) {
				require.NoError(t,
					Waiter.WaitForCondition(ctx,
						&currObjectSet,
						corev1alpha1.ObjectSetArchived,
						metav1.ConditionTrue,
					),
				)
				availableCond := meta.FindStatusCondition(currObjectSet.Status.Conditions, corev1alpha1.ObjectSetArchived)
				require.NotNil(t, availableCond, "Available condition is expected to be reported")
				require.Equal(t, metav1.ConditionTrue, availableCond.Status)
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

func ExpectedObjectSetName(deployment *corev1alpha1.ObjectDeployment) string {
	return fmt.Sprintf("%s-%s", deployment.GetName(), deployment.Status.TemplateHash)
}

func objectDeploymentTemplate(
	objectSetPhases []corev1alpha1.ObjectSetTemplatePhase,
	probes []corev1alpha1.ObjectSetProbe, name string,
) *corev1alpha1.ObjectDeployment {
	label := "test.package-operator.run/" + name
	return &corev1alpha1.ObjectDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectDeploymentSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{label: "True"},
			},
			Template: corev1alpha1.ObjectSetTemplate{
				Metadata: metav1.ObjectMeta{
					Labels: map[string]string{label: "True"},
				},
				Spec: corev1alpha1.ObjectSetTemplateSpec{
					Phases:             objectSetPhases,
					AvailabilityProbes: probes,
				},
			},
		},
	}
}

func cmTemplate(name string, data map[string]string, t require.TestingT) unstructured.Unstructured {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"test.package-operator.run/test-1": "True"},
		},
		Data: data,
	}
	GVK, err := apiutil.GVKForObject(&cm, Scheme)
	require.NoError(t, err)
	cm.SetGroupVersionKind(GVK)

	resObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cm)
	require.NoError(t, err)
	return unstructured.Unstructured{Object: resObj}
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

func deploymentTemplate(deploymentName string, podImage string, t require.TestingT) unstructured.Unstructured {
	obj := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   deploymentName,
			Labels: map[string]string{"test.package-operator.run/test-1": "True"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"test.package-operator.run/test-1": "True"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "nginx",
					Labels: map[string]string{"test.package-operator.run/test-1": "True"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: podImage,
						},
					},
				},
			},
		},
	}

	GVK, err := apiutil.GVKForObject(&obj, Scheme)
	require.NoError(t, err)
	obj.SetGroupVersionKind(GVK)
	resObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	require.NoError(t, err)
	return unstructured.Unstructured{Object: resObj}
}

func archivalTestprobesTemplate(configmapFieldA, configmapFieldB string) []corev1alpha1.ObjectSetProbe {
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
						FieldA: configmapFieldA,
						FieldB: configmapFieldB,
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

func hashCollisionTestProbe(configmapFieldA, configmapFieldB string) []corev1alpha1.ObjectSetProbe {
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
						FieldA: configmapFieldA,
						FieldB: configmapFieldB,
					},
				},
			},
		},
	}
}
