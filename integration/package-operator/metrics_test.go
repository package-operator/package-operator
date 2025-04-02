//go:build integration

package packageoperator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
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
	found, err := testutil.VerifyMetrics(ctx, "package_availability", "pko_name", packageName)
	require.NoError(t, err)
	require.True(t, found)

	// Get the package and then delete it
	require.NoError(t, Client.Get(ctx, client.ObjectKey{Name: packageName}, pkg))
	require.NoError(t, Client.Delete(ctx, pkg))
	require.NoError(t, Waiter.WaitToBeGone(ctx, pkg, func(client.Object) (bool, error) { return false, nil }))

	// Get the metrics again and verify there's no vector for the package
	found, err = testutil.VerifyMetrics(ctx, "package_availability", "pko_name", packageName)
	require.NoError(t, err)
	require.False(t, found)
}

func TestObjectSetMetrics_RevisionHistoryGreaterThanZero(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	probe := hashCollisionTestProbe(".metadata.name", ".data.name")
	phases := []corev1alpha1.ObjectSetTemplatePhase{
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
	}

	// Create object deployment with revision history limit of 1
	objectDeployment := objectDeploymentTemplate(phases, probe, "test-objectdeployment", 1)
	require.NoError(t, Client.Create(ctx, objectDeployment), "error creating object set")
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))
	cleanupOnSuccess(ctx, t, objectDeployment)

	objectSetList := listObjectSetRevisions(ctx, t, objectDeployment)
	objset := objectSetList.Items[0]

	found, err := testutil.VerifyMetrics(ctx, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	require.True(t, found)

	// Update the objectdeployment to create revion 2
	phases = []corev1alpha1.ObjectSetTemplatePhase{
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
	}

	objectDeployment.Spec.Template.Spec.Phases = phases
	require.NoError(t, Client.Update(ctx, objectDeployment))
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentProgressing, metav1.ConditionFalse))
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))

	// Verify metrics are still there for revision 1
	found, err = testutil.VerifyMetrics(ctx, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	require.True(t, found)

	// Update the objectdeployment to create revision 3
	phases = []corev1alpha1.ObjectSetTemplatePhase{
		{
			Name: "phase-1",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: cmTemplate("cm3", map[string]string{"name": "cm3"}, t),
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
	}

	objectDeployment.Spec.Template.Spec.Phases = phases
	require.NoError(t, Client.Update(ctx, objectDeployment))
	require.NoError(t,
		Waiter.WaitToBeGone(ctx, &objset, func(client.Object) (bool, error) { return false, nil }))

	// Verify there are no metrics for revision 1
	found, err = testutil.VerifyMetrics(ctx, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	require.False(t, found)
}

func TestObjectSetMetrics_RevisionHistoryZero(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	probe := hashCollisionTestProbe(".metadata.name", ".data.name")
	phases := []corev1alpha1.ObjectSetTemplatePhase{
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
	}

	// Create object deployment with revision history limit of 1
	objectDeployment := objectDeploymentTemplate(phases, probe, "test-objectdeployment", 0)
	require.NoError(t, Client.Create(ctx, objectDeployment), "error creating object set")
	require.NoError(t,
		Waiter.WaitForCondition(ctx, objectDeployment, corev1alpha1.ObjectDeploymentAvailable, metav1.ConditionTrue))
	cleanupOnSuccess(ctx, t, objectDeployment)

	objectSetList := listObjectSetRevisions(ctx, t, objectDeployment)
	objset := objectSetList.Items[0]
	found, err := testutil.VerifyMetrics(ctx, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	require.True(t, found)

	// Update the objectdeployment to create revion 2
	phases = []corev1alpha1.ObjectSetTemplatePhase{
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
	}

	objectDeployment.Spec.Template.Spec.Phases = phases
	require.NoError(t, Client.Update(ctx, objectDeployment))
	require.NoError(t, Waiter.WaitToBeGone(ctx, &objset, func(client.Object) (bool, error) { return false, nil }))

	// Verify there are no metrics for revision 1
	found, err = testutil.VerifyMetrics(ctx, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	require.False(t, found)
}

func TestObjectSetMetrics_ObjectDeploymentDeleted(t *testing.T) {
	ctx := logr.NewContext(context.Background(), testr.New(t))

	probe := hashCollisionTestProbe(".metadata.name", ".data.name")
	phases := []corev1alpha1.ObjectSetTemplatePhase{
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
	found, err := testutil.VerifyMetrics(ctx, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	require.True(t, found)

	// Delete the objectdeployment
	require.NoError(t, Client.Delete(ctx, objectDeployment))
	require.NoError(t, Waiter.WaitToBeGone(ctx, &objset, func(client.Object) (bool, error) { return false, nil }))

	// Verify there are no metrics for the objectset
	found, err = testutil.VerifyMetrics(ctx, "object_set_created_timestamp_seconds", "pko_name", objset.Name)
	require.NoError(t, err)
	require.False(t, found)
}
