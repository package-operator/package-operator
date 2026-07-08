//go:build integration

package packageoperator

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"pkg.package-operator.run/cardboard/kubeutils/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
)

func TestCollisionPrevention(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runCollisionPreventionTests(t, "default", test.class)
		})
	}
}

// Simple Setup, Pause and Teardown test.
func TestObjectSet_setupPauseTeardown(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetSetupPauseTeardownTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_handover(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetHandoverTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_orphanCascadeDeletion(t *testing.T) {
	tests := []struct {
		name  string
		class string
	}{
		{
			name:  "run without objectsetphase objects",
			class: "",
		},
		{
			name:  "run with sameclusterobjectsetphasecontroller",
			class: "default",
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			runObjectSetOrphanCascadeDeletionTest(t, "default", test.class)
		})
	}
}

func TestObjectSet_teardownObjectNotControlledAnymore(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-teardown-uncontrolled",
			Namespace: "default",
		},
		Data: map[string]string{
			"banana":       "bread",
			"uncontrolled": "emotions",
		},
	}

	cmGVK, err := apiutil.GVKForObject(configMap, Scheme)
	require.NoError(t, err)
	configMap.SetGroupVersionKind(cmGVK)

	unstructuredCM, err := runtime.DefaultUnstructuredConverter.ToUnstructured(configMap)
	require.NoError(t, err)

	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-teardown-uncontrolled",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			Revision: 1,
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{
					{
						Name:  "phase-1",
						Class: "default",
						Objects: []corev1alpha1.ObjectSetObject{
							{
								CollisionProtection: "Prevent",
								Object:              unstructured.Unstructured{Object: unstructuredCM},
							},
						},
					},
				},
			},
		},
	}

	// Apply ObjectSet and wait for it to become available.
	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)
	require.NoError(t, Waiter.WaitForCondition(ctx, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue))

	// Fetch ConfigMap from API and disable the controller flag on its owner reference.
	actualConfigMap := &corev1.ConfigMap{}
	require.NoError(t, Client.Get(ctx, client.ObjectKeyFromObject(configMap), actualConfigMap))
	actualConfigMap.OwnerReferences[0].Controller = new(false)
	require.NoError(t, Client.Update(ctx, actualConfigMap))

	// Delete ObjectSet.
	require.NoError(t, Client.Delete(ctx, objectSet))

	// Wait for owner reference on ConfigMap to be removed.
	require.NoError(t,
		Waiter.WaitForObject(
			ctx, actualConfigMap, "internal ownerReference to be removed",
			func(client.Object) (bool, error) {
				configMap := &corev1.ConfigMap{}
				err := Client.Get(ctx, client.ObjectKeyFromObject(actualConfigMap), configMap)
				ownerRefFound := false
				for _, owner := range configMap.GetOwnerReferences() {
					if owner.Name == objectSet.Name {
						ownerRefFound = true
					}
				}
				return !ownerRefFound, err
			}, wait.WithTimeout(40*time.Second),
		))
}

func TestObjectSet_immutability(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	configMap := cmTemplate("test-immutability", "", map[string]string{"banana": "bread"}, t)

	objectSet := &corev1alpha1.ObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-immutability",
			Namespace: "default",
		},
		Spec: corev1alpha1.ObjectSetSpec{
			Revision: 1,
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{{
						CollisionProtection: "Prevent",
						Object:              configMap,
					}},
				}},
			},
		},
	}
	require.NoError(t, Client.Create(ctx, objectSet))
	cleanupOnSuccess(ctx, t, objectSet)
	requireCondition(ctx, t, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue)

	clusterConfigMap := cmTemplate("cl-test-immutability", "default", map[string]string{"banana": "bread"}, t)

	clusterObjectSet := &corev1alpha1.ClusterObjectSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cl-test-immutability",
			Namespace: "default",
		},
		Spec: corev1alpha1.ClusterObjectSetSpec{
			Revision: 1,
			ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
				Phases: []corev1alpha1.ObjectSetTemplatePhase{{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{{
						CollisionProtection: "Prevent",
						Object:              clusterConfigMap,
					}},
				}},
			},
		},
	}
	require.NoError(t, Client.Create(ctx, clusterObjectSet))
	cleanupOnSuccess(ctx, t, clusterObjectSet)
	requireCondition(ctx, t, clusterObjectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue)

	for _, tc := range []struct {
		field  string
		modify func(adapters.ObjectSetAccessor)
	}{
		{
			field: "phases",
			modify: func(os adapters.ObjectSetAccessor) {
				os.SetSpecPhases([]corev1alpha1.ObjectSetTemplatePhase{})
			},
		},
		{
			field: "availabilityProbes",
			modify: func(os adapters.ObjectSetAccessor) {
				ts := os.GetSpecTemplateSpec()
				ts.AvailabilityProbes = append(
					ts.AvailabilityProbes,
					corev1alpha1.ObjectSetProbe{
						Probes: []corev1alpha1.Probe{{
							Condition: &corev1alpha1.ProbeConditionSpec{
								Type:   "Available",
								Status: "True",
							},
						}},
						Selector: corev1alpha1.ProbeSelector{
							Kind: &corev1alpha1.PackageProbeKindSpec{
								Group: "v1",
								Kind:  "ConfigMap",
							},
						},
					},
				)
				os.SetSpecTemplateSpec(ts)
			},
		},
		{
			field: "successDelaySeconds",
			modify: func(os adapters.ObjectSetAccessor) {
				ts := os.GetSpecTemplateSpec()
				ts.SuccessDelaySeconds += 42
				os.SetSpecTemplateSpec(ts)
			},
		},
		{
			field: "previous",
			modify: func(os adapters.ObjectSetAccessor) {
				os.SetSpecPreviousRevisions([]adapters.ObjectSetAccessor{os})
			},
		},
		{
			field: "revision",
			modify: func(os adapters.ObjectSetAccessor) {
				os.SetSpecRevision(os.GetSpecRevision() + 42)
			},
		},
	} {
		t.Run(tc.field, func(t *testing.T) {
			newObjectSet := objectSet.DeepCopy()
			newObjectSetAdapter := &adapters.ObjectSetAdapter{
				ObjectSet: *newObjectSet,
			}
			tc.modify(newObjectSetAdapter)
			require.ErrorContains(t, Client.Update(ctx, &newObjectSetAdapter.ObjectSet), tc.field+" is immutable")
		})
		t.Run(tc.field+"-cluster", func(t *testing.T) {
			newObjectSet := clusterObjectSet.DeepCopy()
			newObjectSetAdapter := &adapters.ClusterObjectSetAdapter{
				ClusterObjectSet: *newObjectSet,
			}
			tc.modify(newObjectSetAdapter)
			require.ErrorContains(t, Client.Update(ctx, &newObjectSetAdapter.ClusterObjectSet), tc.field+" is immutable")
		})
	}
}

func TestObjectSet_invalidPreviousReference(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	configMap := cmTemplate("test-invalid-previous-reference", "default", map[string]string{"banana": "bread"}, t)
	objectSetTemplateSpec := corev1alpha1.ObjectSetTemplateSpec{
		Phases: []corev1alpha1.ObjectSetTemplatePhase{{
			Name: "phase-1",
			Objects: []corev1alpha1.ObjectSetObject{{
				CollisionProtection: "Prevent",
				Object:              configMap,
			}},
		}},
	}

	t.Run("namespaced", func(t *testing.T) {
		prev := &corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "previous-revision",
				Namespace: "default",
				Labels: map[string]string{
					controllers.DeploymentLabel: "test-deployment",
				},
			},
			Spec: corev1alpha1.ObjectSetSpec{
				ObjectSetTemplateSpec: objectSetTemplateSpec,
				Revision:              1,
			},
		}

		objectSet := prev.DeepCopy()
		objectSet.Name = "test-invalid-previous-reference"
		objectSet.Spec.Previous = []corev1alpha1.PreviousRevisionReference{
			{Name: prev.Name},
			{Name: "non-existent-revision"},
		}

		// Create previous ObjectSet
		require.NoError(t, Client.Create(ctx, prev))
		cleanupOnSuccess(ctx, t, prev)
		requireCondition(ctx, t, prev, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue)

		// Create new ObjectSet with reference to previous and non-existent
		objectSet.Spec.Revision = prev.Spec.Revision + 1
		require.NoError(t, Client.Create(ctx, objectSet))
		cleanupOnSuccess(ctx, t, objectSet)
		requireCondition(ctx, t, objectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue)
		assert.Equal(t, prev.Spec.Revision+1, objectSet.Spec.Revision)
	})

	t.Run("cluster", func(t *testing.T) {
		prev := &corev1alpha1.ClusterObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "previous-revision",
				Namespace: "default",
				Labels: map[string]string{
					controllers.DeploymentLabel: "test-deployment",
				},
			},
			Spec: corev1alpha1.ClusterObjectSetSpec{
				ObjectSetTemplateSpec: objectSetTemplateSpec,
				Revision:              1,
			},
		}

		clusterObjectSet := prev.DeepCopy()
		clusterObjectSet.Name = "test-invalid-previous-reference"
		clusterObjectSet.Spec.Previous = []corev1alpha1.PreviousRevisionReference{
			{Name: prev.Name},
			{Name: "non-existent-revision"},
		}

		// Create previous ClusterObjectSet
		require.NoError(t, Client.Create(ctx, prev))
		cleanupOnSuccess(ctx, t, prev)
		requireCondition(ctx, t, prev, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue)

		// Create new ClusterObjectSet with reference to previous and non-existent
		clusterObjectSet.Spec.Revision = prev.Spec.Revision + 1
		require.NoError(t, Client.Create(ctx, clusterObjectSet))
		cleanupOnSuccess(ctx, t, clusterObjectSet)
		requireCondition(ctx, t, clusterObjectSet, corev1alpha1.ObjectSetAvailable, metav1.ConditionTrue)
		assert.Equal(t, prev.Spec.Revision+1, clusterObjectSet.Spec.Revision)
	})
}
