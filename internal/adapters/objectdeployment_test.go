package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

func TestObjectDeployment(t *testing.T) {
	deploy := NewObjectDeployment(testScheme).(*ObjectDeployment)

	co := deploy.ClientObject()
	assert.IsType(t, &corev1alpha1.ObjectDeployment{}, co)

	var revisionHistoryLimit int32 = 2
	deploy.ObjectDeployment.Spec.RevisionHistoryLimit = &revisionHistoryLimit
	assert.Equal(t, &revisionHistoryLimit, deploy.GetRevisionHistoryLimit())

	var collisionCount int32 = 4
	deploy.SetStatusCollisionCount(&collisionCount)
	assert.Equal(t, &collisionCount, deploy.GetStatusCollisionCount())

	deploy.Status.Conditions = []metav1.Condition{}
	assert.Equal(t, deploy.Status.Conditions, *deploy.GetConditions())

	deploy.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{"test": "test"},
	}
	assert.Equal(t, deploy.Spec.Selector, deploy.GetSelector())

	deploy.Spec.Template = corev1alpha1.ObjectSetTemplate{}
	assert.Equal(t, deploy.Spec.Template, deploy.GetObjectSetTemplate())
	deploy.SetTemplateSpec(corev1alpha1.ObjectSetTemplateSpec{})
	assert.Equal(t, deploy.Spec.Template.Spec, deploy.GetTemplateSpec())

	templateHash := "hash123"
	deploy.SetStatusTemplateHash(templateHash)
	assert.Equal(t, templateHash, deploy.GetStatusTemplateHash())

	selector := map[string]string{
		"a": "b",
	}
	deploy.SetSelector(selector)
	assert.Equal(t, selector, deploy.Spec.Selector.MatchLabels)
	assert.Equal(t, selector, deploy.Spec.Template.Metadata.Labels)
}

func TestClusterObjectDeployment(t *testing.T) {
	deploy := NewClusterObjectDeployment(testScheme).(*ClusterObjectDeployment)

	co := deploy.ClientObject()
	assert.IsType(t, &corev1alpha1.ClusterObjectDeployment{}, co)

	var revisionHistoryLimit int32 = 2
	deploy.ClusterObjectDeployment.Spec.RevisionHistoryLimit = &revisionHistoryLimit
	assert.Equal(t, &revisionHistoryLimit, deploy.GetRevisionHistoryLimit())

	var collisionCount int32 = 4
	deploy.SetStatusCollisionCount(&collisionCount)
	assert.Equal(t, &collisionCount, deploy.GetStatusCollisionCount())

	deploy.Status.Conditions = []metav1.Condition{}
	assert.Equal(t, deploy.Status.Conditions, *deploy.GetConditions())

	deploy.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{"test": "test"},
	}
	assert.Equal(t, deploy.Spec.Selector, deploy.GetSelector())

	deploy.Spec.Template = corev1alpha1.ObjectSetTemplate{}
	assert.Equal(t, deploy.Spec.Template, deploy.GetObjectSetTemplate())
	deploy.SetTemplateSpec(corev1alpha1.ObjectSetTemplateSpec{})
	assert.Equal(t, deploy.Spec.Template.Spec, deploy.GetTemplateSpec())

	templateHash := "hash123"
	deploy.SetStatusTemplateHash(templateHash)
	assert.Equal(t, templateHash, deploy.GetStatusTemplateHash())

	selector := map[string]string{
		"a": "b",
	}
	deploy.SetSelector(selector)
	assert.Equal(t, selector, deploy.Spec.Selector.MatchLabels)
	assert.Equal(t, selector, deploy.Spec.Template.Metadata.Labels)
}

func Test_objectDeploymentPhase(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		expectedPhase corev1alpha1.ObjectDeploymentPhase
	}{
		{
			name: "Available",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.ObjectDeploymentAvailable,
					Status: metav1.ConditionTrue,
				},
			},
			expectedPhase: corev1alpha1.ObjectDeploymentPhaseAvailable,
		},
		{
			name: "NotReady",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.ObjectDeploymentAvailable,
					Status: metav1.ConditionFalse,
				},
			},
			expectedPhase: corev1alpha1.ObjectDeploymentPhaseNotReady,
		},
		{
			name: "Progressing",
			conditions: []metav1.Condition{
				{
					Type:   corev1alpha1.ObjectDeploymentProgressing,
					Status: metav1.ConditionFalse,
				},
			},
			expectedPhase: corev1alpha1.ObjectDeploymentPhaseProgressing,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			phase := objectDeploymentPhase(test.conditions)
			assert.Equal(t, test.expectedPhase, phase)
		})
	}
}
