//go:build integration

package packageoperator

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

func TestPackageMetrics_PackageDeleted(t *testing.T) {
	packageName := "test-package"
	ctx := logr.NewContext(context.Background(), testr.New(t))

	// Deploy a package and verify the metrics exist
	pkg := clusterPackageTemplate(packageName)
	requireDeployPackage(ctx, t, pkg, &corev1alpha1.ClusterObjectDeployment{})
	found, err := testutil.MetricsVectorExists(ctx, Config, "package_availability", "pko_name", packageName)
	require.NoError(t, err)
	assert.True(t, found)

	// Delete the package
	require.NoError(t, Client.Delete(ctx, pkg))
	require.NoError(t, Waiter.WaitToBeGone(ctx, pkg, func(client.Object) (bool, error) { return false, nil }))

	// Get the metrics again and verify there's no vector for the package
	found, err = testutil.MetricsVectorExists(ctx, Config, "package_availability", "pko_name", packageName)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestObjectSetMetrics_ObjectSetsGarbageCollected(t *testing.T) {
	for revisionHistoryLimit := range 2 {
		t.Run(fmt.Sprintf("RevisionHistoryLimit%d", revisionHistoryLimit), func(t *testing.T) {
			ctx := logr.NewContext(context.Background(), testr.New(t))
			phases := []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate(fmt.Sprintf("cm1-%d", revisionHistoryLimit), "", map[string]string{"name": "cm1"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate(fmt.Sprintf("cm2-%d", revisionHistoryLimit), "", map[string]string{"name": "cm2"}, t),
						},
					},
				},
			}

			// Create object deployment
			objectDeployment := objectDeploymentTemplate(
				phases, nil, fmt.Sprintf("test-objectdeployment-%d", revisionHistoryLimit), int32(revisionHistoryLimit))
			require.NoError(t, Client.Create(ctx, objectDeployment), "error creating object deployment")
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))
			cleanupOnSuccess(ctx, t, objectDeployment)

			objectSetList := listObjectSetRevisions(ctx, t, objectDeployment)
			objset := objectSetList.Items[0]

			// Verify the metrics exists for revision 1
			found, err := testutil.MetricsVectorExists(ctx, Config,
				"object_set_created_timestamp_seconds", "pko_name", objset.Name)
			require.NoError(t, err)
			assert.True(t, found)

			// Update the objectdeployment to create revision 2
			phases = []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate(fmt.Sprintf("cm1-%d", revisionHistoryLimit), "", map[string]string{"name": "cm1"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: deploymentTemplate(fmt.Sprintf("nginx1-%d", revisionHistoryLimit), "nginx:1.14.2", t),
						},
					},
				},
			}

			objectDeployment.Spec.Template.Spec.Phases = phases
			require.NoError(t, Client.Update(ctx, objectDeployment))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentProgressing, metav1.ConditionFalse))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))

			// Verify metrics for revision 1
			found, err = testutil.MetricsVectorExists(ctx, Config,
				"object_set_created_timestamp_seconds", "pko_name", objset.Name)
			require.NoError(t, err)
			if revisionHistoryLimit > 0 {
				// If revision history is 1, the metrics should still exists
				assert.True(t, found)
			} else {
				// If revision history is 0, the metrics should have been garbage collected
				assert.False(t, found)
			}

			// Update the objectdeployment to create revision 3
			phases = []corev1alpha1.ObjectSetTemplatePhase{
				{
					Name: "phase-1",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: cmTemplate(fmt.Sprintf("cm3-%d", revisionHistoryLimit), "", map[string]string{"name": "cm3"}, t),
						},
					},
				},
				{
					Name: "phase-2",
					Objects: []corev1alpha1.ObjectSetObject{
						{
							Object: deploymentTemplate(fmt.Sprintf("nginx1-%d", revisionHistoryLimit), "nginx:1.14.2", t),
						},
					},
				},
			}

			objectDeployment.Spec.Template.Spec.Phases = phases
			require.NoError(t, Client.Update(ctx, objectDeployment))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentProgressing, metav1.ConditionFalse))
			require.NoError(t,
				Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))

			// Verify there are no metrics for revision 1
			found, err = testutil.MetricsVectorExists(ctx, Config,
				"object_set_created_timestamp_seconds", "pko_name", objset.Name)
			require.NoError(t, err)
			assert.False(t, found)
		})
	}
}

func TestObjectSetMetrics_ObjectDeploymentDeleted(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	probe := hashCollisionTestProbe()
	phases := []corev1alpha1.ObjectSetTemplatePhase{
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
	}

	// Create object deployment
	objectDeployment := objectDeploymentTemplate(phases, probe, "test-objectdeployment", 10)
	require.NoError(t, Client.Create(ctx, objectDeployment), "error creating object set")
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))
	cleanupOnSuccess(ctx, t, objectDeployment)

	// Verify metrics exist for the objectset
	objectSetList := listObjectSetRevisions(ctx, t, objectDeployment)
	objset := objectSetList.Items[0]
	found, err := testutil.MetricsVectorExists(ctx, Config,
		"object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	assert.True(t, found)

	// Delete the objectdeployment
	require.NoError(t, Client.Delete(ctx, objectDeployment))
	require.NoError(t, Waiter.WaitToBeGone(ctx, &objset, func(client.Object) (bool, error) { return false, nil }))

	// Verify there are no metrics for the objectset
	found, err = testutil.MetricsVectorExists(ctx, Config, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestManagedCacheMetrics(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	found, err := testutil.MetricsVectorExists(ctx, Config, "managed_cache_informers_total", "owner", "123-456")
	require.NoError(t, err)
	assert.True(t, found)

	deployments := deploymentsInCache(ctx, t)

	pkg := clusterPackageTemplate("test-package")
	requireDeployPackage(ctx, t, pkg, &corev1alpha1.ClusterObjectDeployment{})

	// Expect one more deployment after the test-stub package is available
	assert.Equal(t, deployments+1, deploymentsInCache(ctx, t))

	require.NoError(t, Client.Delete(ctx, pkg))
	require.NoError(t, Waiter.WaitToBeGone(ctx, pkg, func(client.Object) (bool, error) { return false, nil }))

	// After deleting the package, the number of deployments should be back to the original value
	assert.Equal(t, deployments, deploymentsInCache(ctx, t))
}

func deploymentsInCache(ctx context.Context, t *testing.T) int {
	t.Helper()

	metric, err := testutil.GetMetric(ctx, Config, "managed_cache_objects_total", "gvk", "apps/v1, Kind=Deployment")
	require.NoError(t, err)
	return int(metric.GetGauge().GetValue())
}
