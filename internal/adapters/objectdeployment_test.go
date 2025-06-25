package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectDeployment(t *testing.T) {
	t.Parallel()

	deploy := NewObjectDeployment(testScheme).(*ObjectDeployment)

	co := deploy.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectDeployment{}, co)

	var revisionHistoryLimit int32 = 2
	deploy.Spec.RevisionHistoryLimit = &revisionHistoryLimit
	assert.Equal(t, &revisionHistoryLimit, deploy.GetSpecRevisionHistoryLimit())

	var collisionCount int32 = 4
	deploy.SetStatusCollisionCount(&collisionCount)
	assert.Equal(t, &collisionCount, deploy.GetStatusCollisionCount())

	deploy.Status.Conditions = []metav1.Condition{}
	assert.Equal(t, deploy.Status.Conditions, *deploy.GetStatusConditions())

	deploy.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{"test": "test"},
	}
	assert.Equal(t, deploy.Spec.Selector, deploy.GetSpecSelector())

	deploy.Spec.Template = corev1alpha1.ObjectSetTemplate{}
	assert.Equal(t, deploy.Spec.Template, deploy.GetSpecObjectSetTemplate())
	deploy.SetSpecTemplateSpec(corev1alpha1.ObjectSetTemplateSpec{})
	assert.Equal(t, deploy.Spec.Template.Spec, deploy.GetSpecTemplateSpec())

	templateHash := "hash123"
	deploy.SetStatusTemplateHash(templateHash)
	assert.Equal(t, templateHash, deploy.GetStatusTemplateHash())

	selector := map[string]string{
		"a": "b",
	}
	deploy.SetSpecSelector(selector)
	assert.Equal(t, selector, deploy.Spec.Selector.MatchLabels)
	assert.Equal(t, selector, deploy.Spec.Template.Metadata.Labels)

	var statusRevision int64 = 2
	deploy.SetStatusRevision(statusRevision)
	assert.Equal(t, statusRevision, deploy.GetStatusRevision())

	deploy.SetSpecPaused(true)
	assert.True(t, deploy.GetSpecPaused())
	deploy.SetSpecPaused(false)
	assert.False(t, deploy.GetSpecPaused())

	condition := metav1.Condition{
		Type: "test-condition",
	}
	deploy.Status.Conditions = []metav1.Condition{condition}
	deploy.RemoveStatusConditions(condition.Type)
	assert.Empty(t, deploy.Status.Conditions)
}

func TestClusterObjectDeployment(t *testing.T) {
	t.Parallel()
	deploy := NewClusterObjectDeployment(testScheme).(*ClusterObjectDeployment)

	co := deploy.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectDeployment{}, co)

	var revisionHistoryLimit int32 = 2
	deploy.Spec.RevisionHistoryLimit = &revisionHistoryLimit
	assert.Equal(t, &revisionHistoryLimit, deploy.GetSpecRevisionHistoryLimit())

	var collisionCount int32 = 4
	deploy.SetStatusCollisionCount(&collisionCount)
	assert.Equal(t, &collisionCount, deploy.GetStatusCollisionCount())

	deploy.Status.Conditions = []metav1.Condition{}
	assert.Equal(t, deploy.Status.Conditions, *deploy.GetStatusConditions())

	deploy.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{"test": "test"},
	}
	assert.Equal(t, deploy.Spec.Selector, deploy.GetSpecSelector())

	deploy.Spec.Template = corev1alpha1.ObjectSetTemplate{}
	assert.Equal(t, deploy.Spec.Template, deploy.GetSpecObjectSetTemplate())
	deploy.SetSpecTemplateSpec(corev1alpha1.ObjectSetTemplateSpec{})
	assert.Equal(t, deploy.Spec.Template.Spec, deploy.GetSpecTemplateSpec())

	templateHash := "hash123"
	deploy.SetStatusTemplateHash(templateHash)
	assert.Equal(t, templateHash, deploy.GetStatusTemplateHash())

	selector := map[string]string{
		"a": "b",
	}
	deploy.SetSpecSelector(selector)
	assert.Equal(t, selector, deploy.Spec.Selector.MatchLabels)
	assert.Equal(t, selector, deploy.Spec.Template.Metadata.Labels)

	var statusRevision int64 = 2
	deploy.SetStatusRevision(statusRevision)
	assert.Equal(t, statusRevision, deploy.GetStatusRevision())

	deploy.SetSpecPaused(true)
	assert.True(t, deploy.GetSpecPaused())
	deploy.SetSpecPaused(false)
	assert.False(t, deploy.GetSpecPaused())

	condition := metav1.Condition{
		Type: "test-condition",
	}
	deploy.Status.Conditions = []metav1.Condition{condition}
	deploy.RemoveStatusConditions(condition.Type)
	assert.Empty(t, deploy.Status.Conditions)
}
