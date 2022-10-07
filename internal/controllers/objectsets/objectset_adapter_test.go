package objectsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

var tests = []struct {
	name                string
	startConditions     []metav1.Condition
	expectedStatusPhase corev1alpha1.ObjectSetStatusPhase
}{
	{
		name: "archived true",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetArchived,
				Status: metav1.ConditionTrue,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseArchived,
	},
	{
		name: "archived false",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetArchived,
				Status: metav1.ConditionFalse,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseNotReady,
	},
	{
		name: "paused true",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetPaused,
				Status: metav1.ConditionTrue,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhasePaused,
	},
	{
		name: "available true",
		startConditions: []metav1.Condition{
			{
				Type:   corev1alpha1.ObjectSetAvailable,
				Status: metav1.ConditionTrue,
			},
		},
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseAvailable,
	},
	{
		name:                "no conditions",
		expectedStatusPhase: corev1alpha1.ObjectSetStatusPhaseNotReady,
	},
}

func TestGenericObjectSet_UpdateStatusPhase(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clusterObjectSet := GenericObjectSet{}
			clusterObjectSet.Status.Conditions = test.startConditions
			clusterObjectSet.UpdateStatusPhase()
			assert.Equal(t, test.expectedStatusPhase, clusterObjectSet.Status.Phase)
		})
	}
}

func TestGenericClusterObjectSet_UpdateStatusPhase(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clusterObjectSet := GenericClusterObjectSet{}
			clusterObjectSet.Status.Conditions = test.startConditions
			clusterObjectSet.UpdateStatusPhase()
			assert.Equal(t, test.expectedStatusPhase, clusterObjectSet.Status.Phase)
		})
	}
}
