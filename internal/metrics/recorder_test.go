package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers/objectsets"
)

func TestRecorder_RecordRolloutTime(t *testing.T) {
	// Difference of 33 minutes and 17 seconds
	creationTimestamp := time.Date(2022, 5, 27, 15, 4, 2, 0, time.UTC)
	lastTransitionTime := time.Date(2022, 5, 27, 15, 37, 19, 0, time.UTC)

	objectSet := &objectsets.GenericObjectSet{
		ObjectSet: corev1alpha1.ObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(creationTimestamp),
			},
			Status: corev1alpha1.ObjectSetStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Success",
						LastTransitionTime: metav1.NewTime(lastTransitionTime),
					},
				},
			},
		},
	}
	recorder := NewRecorder(false)
	recorder.RecordRolloutTime(objectSet)
	assert.Equal(t, float64(33*60+17), testutil.ToFloat64(recorder.rolloutTime))
}
