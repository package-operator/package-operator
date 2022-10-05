package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type genericObjectSetMock struct {
	mock.Mock
}

func (m *genericObjectSetMock) ClientObject() client.Object {
	args := m.Called()
	return args.Get(0).(client.Object)
}

func (m *genericObjectSetMock) GetConditions() *[]metav1.Condition {
	args := m.Called()
	return args.Get(0).(*[]metav1.Condition)
}

type genericObjectSetPhaseMock struct {
	mock.Mock
}

func (m *genericObjectSetPhaseMock) ClientObject() client.Object {
	args := m.Called()
	return args.Get(0).(client.Object)
}

func (m *genericObjectSetPhaseMock) GetConditions() *[]metav1.Condition {
	args := m.Called()
	return args.Get(0).(*[]metav1.Condition)
}

func TestRecorder_RecordRolloutTimeObjectSet(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
	}{
		{
			name:       "no success condition",
			conditions: []metav1.Condition{},
		},
		{
			name: "with success condition",
			conditions: []metav1.Condition{
				{
					Type: "Success",
					LastTransitionTime: metav1.NewTime(
						time.Date(2022, 5, 27, 15, 37, 19, 0, time.UTC)),
					// Difference of 33 minutes and 17 seconds from `creationTimestamp`
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			creationTimestamp := time.Date(2022, 5, 27, 15, 4, 2, 0, time.UTC)

			obj := &unstructured.Unstructured{}
			obj.SetCreationTimestamp(metav1.NewTime(creationTimestamp))

			osMock := &genericObjectSetMock{}
			osMock.On("ClientObject").Return(obj)

			// Object does not have successful status condition yet, so nothing should be recorded
			osMock.On("GetConditions").Return(&test.conditions).Once()

			recorder := NewRecorder(false)
			recorder.RecordRolloutTimeObjectSet(osMock)
			if len(test.conditions) == 0 {
				assert.Equal(t, 0, testutil.CollectAndCount(recorder.objectSetRolloutTime))
			} else {
				assert.Equal(t, float64(33*60+17), testutil.ToFloat64(recorder.objectSetRolloutTime))
			}

		})
	}
}

func TestRecorder_RecordRolloutTimeObjectSetPhase(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
	}{
		{
			name:       "no success condition",
			conditions: []metav1.Condition{},
		},
		{
			name: "with success condition",
			conditions: []metav1.Condition{
				{
					Type: "Success",
					LastTransitionTime: metav1.NewTime(
						time.Date(2022, 5, 27, 15, 37, 19, 0, time.UTC)),
					// Difference of 33 minutes and 17 seconds from `creationTimestamp`
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			creationTimestamp := time.Date(2022, 5, 27, 15, 4, 2, 0, time.UTC)

			obj := &unstructured.Unstructured{}
			obj.SetCreationTimestamp(metav1.NewTime(creationTimestamp))

			ospMock := &genericObjectSetPhaseMock{}
			ospMock.On("ClientObject").Return(obj)

			// Object does not have successful status condition yet, so nothing should be recorded
			ospMock.On("GetConditions").Return(&test.conditions).Once()

			recorder := NewRecorder(false)
			recorder.RecordRolloutTimeObjectSet(ospMock)
			if len(test.conditions) == 0 {
				assert.Equal(t, 0, testutil.CollectAndCount(recorder.objectSetRolloutTime))
			} else {
				assert.Equal(t, float64(33*60+17), testutil.ToFloat64(recorder.objectSetRolloutTime))
			}

		})
	}
}
