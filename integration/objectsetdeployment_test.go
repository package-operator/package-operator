package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers/objectdeployments"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func TestObjectDeployment_availability_and_hash_collision(t *testing.T) {
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cm-1",
			Labels: map[string]string{"test.package-operator.run/test": "True"},
		},
		Data: map[string]string{
			"banana": "bread",
			"name":   "cm-1",
		},
	}
	cmGVK, err := apiutil.GVKForObject(cm1, Scheme)
	require.NoError(t, err)
	cm1.SetGroupVersionKind(cmGVK)

	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cm-2",
		},
	}
	cm2.SetGroupVersionKind(cmGVK)

	deployment1Probe := []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Kind: "ConfigMap",
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
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
	}

	deployment2Probe := []corev1alpha1.ObjectSetProbe{
		{
			Selector: corev1alpha1.ProbeSelector{
				Kind: &corev1alpha1.PackageProbeKindSpec{
					Kind: "ConfigMap",
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
				},
			},
			Probes: []corev1alpha1.Probe{
				{
					FieldsEqual: &corev1alpha1.ProbeFieldsEqualSpec{
						FieldA: ".metadata.name",
						FieldB: ".metadata.annotations.name",
					},
				},
			},
		},
	}

	objectDeployment := func(probes []corev1alpha1.ObjectSetProbe) corev1alpha1.ObjectDeployment {
		return corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-objectset-deployment",
				Namespace: "default",
			},
			Spec: corev1alpha1.ObjectDeploymentSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
				},
				Template: corev1alpha1.ObjectSetTemplate{
					Metadata: metav1.ObjectMeta{
						Labels: map[string]string{"test.package-operator.run/test": "True"},
					},
					Spec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name: "phase-1",
								Objects: []corev1alpha1.ObjectSetObject{
									{
										Object: runtime.RawExtension{
											Object: cm1,
										},
									},
								},
							},
							{
								Name: "phase-2",
								Objects: []corev1alpha1.ObjectSetObject{
									{
										Object: runtime.RawExtension{
											Object: cm2,
										},
									},
								},
							},
						},
						AvailabilityProbes: probes,
					},
				},
			},
		}
	}
	cm1Key := client.ObjectKey{
		Name: cm1.Name, Namespace: "default",
	}
	cm2Key := client.ObjectKey{
		Name: cm2.Name, Namespace: "default",
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))

	testCases := []struct {
		deploymentRevision           int
		objectDeploymentFunc         func(probes []corev1alpha1.ObjectSetProbe) corev1alpha1.ObjectDeployment
		probe                        []corev1alpha1.ObjectSetProbe
		expectedRevisionAvailability metav1.ConditionStatus // Objectset for the current deployment revision availability status
		expectedObjectSetCount       int
		expectedHashCollisionCount   int
		expectedDeploymentConditions map[string]metav1.ConditionStatus
		expectedArchivedRevisions    []string
	}{
		{
			deploymentRevision:           1,
			objectDeploymentFunc:         objectDeployment,
			probe:                        deployment1Probe,
			expectedRevisionAvailability: metav1.ConditionTrue,
			expectedObjectSetCount:       1,
			expectedHashCollisionCount:   0,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable: metav1.ConditionTrue,
			},
		},
		{
			deploymentRevision:           2,
			objectDeploymentFunc:         objectDeployment,
			probe:                        deployment2Probe,
			expectedRevisionAvailability: metav1.ConditionFalse,
			expectedObjectSetCount:       2,
			expectedHashCollisionCount:   0,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionTrue,
			},
		},
		{
			deploymentRevision:           3,
			objectDeploymentFunc:         objectDeployment,
			probe:                        deployment1Probe,
			expectedRevisionAvailability: metav1.ConditionTrue,
			expectedObjectSetCount:       3,
			expectedHashCollisionCount:   1,
			expectedDeploymentConditions: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectDeploymentAvailable:   metav1.ConditionTrue,
				corev1alpha1.ObjectDeploymentProgressing: metav1.ConditionFalse,
			},
			expectedArchivedRevisions: []string{"1", "2"},
		},
	}

	for _, testCase := range testCases {
		concernedDeployment := testCase.objectDeploymentFunc(testCase.probe)
		currentInClusterDeployment := &corev1alpha1.ObjectDeployment{}
		err := Client.Get(ctx, client.ObjectKeyFromObject(&concernedDeployment), currentInClusterDeployment)
		if errors.IsNotFound(err) {
			// Create the deployment
			require.NoError(t, Client.Create(ctx, &concernedDeployment))
		} else {
			// Update the existing deployment
			concernedDeployment.ResourceVersion = currentInClusterDeployment.ResourceVersion
			require.NoError(t, Client.Update(ctx, &concernedDeployment))
		}
		cleanupOnSuccess(ctx, t, &concernedDeployment)

		// Assert that all the expected conditions are reported
		for expectedCond, expectedStatus := range testCase.expectedDeploymentConditions {
			require.NoError(t,
				Waiter.WaitForCondition(ctx,
					&concernedDeployment,
					expectedCond,
					expectedStatus,
				),
			)
			availableCond := meta.FindStatusCondition(concernedDeployment.Status.Conditions, expectedCond)
			require.True(t, availableCond.Status == expectedStatus)
		}

		// objectset for the current deployment revision should be present
		currentObjectSet := &corev1alpha1.ObjectSet{}
		require.NoError(t,
			Client.Get(ctx,
				client.ObjectKey{
					Name:      ExpectedObjectSetName(&concernedDeployment),
					Namespace: concernedDeployment.Namespace},
				currentObjectSet,
			),
		)

		// Assert that the objectset for the current revision has the expected availability status
		require.NoError(t,
			Waiter.WaitForCondition(ctx,
				currentObjectSet,
				corev1alpha1.ObjectSetAvailable,
				testCase.expectedRevisionAvailability,
			),
		)
		availableCond := meta.FindStatusCondition(currentObjectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
		require.NotNil(t, availableCond, "Available condition is expected to be reported")
		require.True(t, availableCond.Status == testCase.expectedRevisionAvailability)

		// Assert that objectset has the revision annotation
		require.True(
			t,
			currentObjectSet.GetAnnotations()[objectdeployments.DeploymentRevisionAnnotation] ==
				fmt.Sprint(concernedDeployment.GetGeneration()),
		)

		// expect cm-1 to be present.
		currentCM1 := &corev1.ConfigMap{}
		require.NoError(t, Client.Get(ctx, cm1Key, currentCM1))

		// expect cm-2 to be present.
		currentCM2 := &corev1.ConfigMap{}
		require.NoError(t, Client.Get(ctx, cm2Key, currentCM2))

		// Expect objectset to be created
		currObjectSetList := &corev1alpha1.ObjectSetList{}
		require.NoError(t, Client.List(ctx, currObjectSetList))

		require.Equal(t, len(currObjectSetList.Items), testCase.expectedObjectSetCount)

		// Assert that the expected revisions are archived (and others active)
		for _, currObjectSet := range currObjectSetList.Items {
			currObjectSetRevision := currObjectSet.GetAnnotations()[objectdeployments.DeploymentRevisionAnnotation]
			if slices.Contains(testCase.expectedArchivedRevisions, currObjectSetRevision) {
				require.True(t, currObjectSet.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived)
			} else {
				require.True(t, currObjectSet.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateActive)
			}
		}

		if testCase.expectedHashCollisionCount > 0 {
			// Expect collision count to be the expected value
			require.True(
				t,
				concernedDeployment.Status.CollisionCount != nil &&
					*concernedDeployment.Status.CollisionCount == int32(testCase.expectedHashCollisionCount),
			)
		}

	}
}

func TestObjectDeployment_objectset_archival(t *testing.T) {
	objectDeploymentTemplate := func(
		objectSetPhases []corev1alpha1.ObjectSetTemplatePhase,
		probes []corev1alpha1.ObjectSetProbe) corev1alpha1.ObjectDeployment {
		return corev1alpha1.ObjectDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-objectset-deployment",
				Namespace: "default",
			},
			Spec: corev1alpha1.ObjectDeploymentSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
				},
				Template: corev1alpha1.ObjectSetTemplate{
					Metadata: metav1.ObjectMeta{
						Labels: map[string]string{"test.package-operator.run/test": "True"},
					},
					Spec: corev1alpha1.ObjectSetTemplateSpec{
						Phases:             objectSetPhases,
						AvailabilityProbes: probes,
					},
				},
			},
		}
	}

	cmTemplate := func(name string, data map[string]string) *corev1.ConfigMap {
		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"test.package-operator.run/test": "True"},
			},
			Data: data,
		}
		cmGVK, err := apiutil.GVKForObject(&cm, Scheme)
		require.NoError(t, err)
		cm.SetGroupVersionKind(cmGVK)
		return &cm
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "secret-1",
			Labels: map[string]string{"test.package-operator.run/test": "True"},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"hello": "world",
		},
	}

	secretGVK, err := apiutil.GVKForObject(&secret, Scheme)
	require.NoError(t, err)
	secret.SetGroupVersionKind(secretGVK)

	deploymentTemplate := func(deploymentName string, podImage string) *appsv1.Deployment {
		obj := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:   deploymentName,
				Labels: map[string]string{"test.package-operator.run/test": "True"},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"test.package-operator.run/test": "True"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "nginx",
						Labels: map[string]string{"test.package-operator.run/test": "True"},
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
		deploymentGVK, err := apiutil.GVKForObject(&obj, Scheme)
		require.NoError(t, err)
		obj.SetGroupVersionKind(deploymentGVK)
		return &obj
	}

	probesTemplate := func(configmapFieldA, configmapFieldB string) []corev1alpha1.ObjectSetProbe {
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
							Type:   "Available",
							Status: "True",
						},
					},
				},
			},
		}
	}

	ctx := logr.NewContext(context.Background(), testr.New(t))

	testCases := []struct {
		revision                     string
		phases                       []corev1alpha1.ObjectSetTemplatePhase
		probes                       []corev1alpha1.ObjectSetProbe
		expectedRevisionAvailability metav1.ConditionStatus // Objectset for the current deployment revision availability status
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
							Object: runtime.RawExtension{
								Object: cmTemplate("cm1", map[string]string{"name": "probe-failure"}),
							},
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: runtime.RawExtension{
								Object: deploymentTemplate("nginx-1", "nginx:1.14.1"),
							},
						},
					},
				},
			},
			probes:                       probesTemplate(".metadata.name", ".data.name"),
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
							Object: runtime.RawExtension{
								Object: cmTemplate("cm1", map[string]string{"name": "cm1"}),
							},
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: runtime.RawExtension{
								Object: deploymentTemplate("nginx-1", "nginx:1.14.2"),
							},
						},
					},
				},
			},
			probes:                       probesTemplate(".metadata.name", ".data.name"),
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
							Object: runtime.RawExtension{
								Object: cmTemplate("cm2", map[string]string{"name": "cm2"}),
							},
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: runtime.RawExtension{
								Object: deploymentTemplate("nginx-2", "invalid-image"),
							},
						},
					},
				},
			},
			probes:                       probesTemplate(".metadata.name", ".data.name"),
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
						{
							Object: runtime.RawExtension{
								Object: deploymentTemplate("nginx-2", "nginx:1.14.2"),
							},
						},
						{
							Object: runtime.RawExtension{
								Object: &secret,
							},
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: runtime.RawExtension{
								Object: cmTemplate("cm3", map[string]string{"name": "probe-failure"}),
							},
						},
					},
				},
			},
			probes:                       probesTemplate(".metadata.name", ".data.name"),
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
							Object: runtime.RawExtension{
								Object: cmTemplate("cm4", map[string]string{"name": "probe-failure"}),
							},
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: runtime.RawExtension{
								Object: deploymentTemplate("nginx-3", "nginx:1.14.2"),
							},
						},
						{
							Object: runtime.RawExtension{
								Object: &secret,
							},
						},
					},
				},
			},
			probes:                       probesTemplate(".metadata.name", ".data.name"),
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
							Object: runtime.RawExtension{
								Object: cmTemplate("cm4", map[string]string{"name": "cm4"}),
							},
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: runtime.RawExtension{
								Object: deploymentTemplate("nginx-3", "nginx:1.14.2"),
							},
						},
						{
							Object: runtime.RawExtension{
								Object: &secret,
							},
						},
					},
				},
			},
			probes:                       probesTemplate(".metadata.name", ".data.name"),
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
		concernedDeployment := objectDeploymentTemplate(testCase.phases, testCase.probes)
		currentInClusterDeployment := &corev1alpha1.ObjectDeployment{}
		err := Client.Get(ctx, client.ObjectKeyFromObject(&concernedDeployment), currentInClusterDeployment)
		if errors.IsNotFound(err) {
			// Create the deployment
			require.NoError(t, Client.Create(ctx, &concernedDeployment))
		} else {
			// Update the existing deployment
			concernedDeployment.ResourceVersion = currentInClusterDeployment.ResourceVersion
			require.NoError(t, Client.Update(ctx, &concernedDeployment))
		}
		cleanupOnSuccess(ctx, t, &concernedDeployment)

		// Assert that all the expected conditions are reported
		for expectedCond, expectedStatus := range testCase.expectedDeploymentConditions {
			require.NoError(t,
				Waiter.WaitForCondition(ctx,
					&concernedDeployment,
					expectedCond,
					expectedStatus,
				),
			)
			availableCond := meta.FindStatusCondition(concernedDeployment.Status.Conditions, expectedCond)
			require.True(t, availableCond.Status == expectedStatus)
		}

		// objectset for the current deployment revision should be present
		currentObjectSet := &corev1alpha1.ObjectSet{}
		require.NoError(t,
			Client.Get(ctx,
				client.ObjectKey{
					Name:      ExpectedObjectSetName(&concernedDeployment),
					Namespace: concernedDeployment.Namespace},
				currentObjectSet,
			),
		)

		// Assert that the objectset for the current revision has the expected availability status
		require.NoError(t,
			Waiter.WaitForCondition(ctx,
				currentObjectSet,
				corev1alpha1.ObjectSetAvailable,
				testCase.expectedRevisionAvailability,
			),
		)
		availableCond := meta.FindStatusCondition(currentObjectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
		require.NotNil(t, availableCond, "Available condition is expected to be reported")
		require.True(t, availableCond.Status == testCase.expectedRevisionAvailability)

		// Assert that objectset has the revision annotation
		require.True(
			t,
			currentObjectSet.GetAnnotations()[objectdeployments.DeploymentRevisionAnnotation] ==
				fmt.Sprint(concernedDeployment.GetGeneration()),
		)

		// Expect objectset to be created
		currObjectSetList := &corev1alpha1.ObjectSetList{}
		require.NoError(t, Client.List(ctx, currObjectSetList))

		require.Equal(t, len(currObjectSetList.Items), testCase.expectedObjectSetCount)

		// Assert that the expected revisions are archived (and others active)
		for _, currObjectSet := range currObjectSetList.Items {
			currObjectSetRevision := currObjectSet.GetAnnotations()[objectdeployments.DeploymentRevisionAnnotation]
			if slices.Contains(testCase.expectedArchivedRevisions, currObjectSetRevision) {
				require.True(t, currObjectSet.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateArchived)
			} else {
				require.True(t, currObjectSet.Spec.LifecycleState == corev1alpha1.ObjectSetLifecycleStateActive)
			}

			// Assert that that expected revision is available
			if currObjectSetRevision == testCase.expectedAvailableRevision {
				availableCond := meta.FindStatusCondition(currObjectSet.Status.Conditions, corev1alpha1.ObjectSetAvailable)
				require.NotNil(t, availableCond, "Available condition is expected to be reported")
				require.True(t, availableCond.Status == metav1.ConditionTrue)
			}
		}
	}
}

func ExpectedObjectSetName(deployment *corev1alpha1.ObjectDeployment) string {
	return fmt.Sprintf("%s-%s", deployment.GetName(), deployment.Status.TemplateHash)
}
