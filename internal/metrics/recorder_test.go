package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
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

func (m *genericObjectSetMock) GetRevision() int64 {
	args := m.Called()
	return args.Get(0).(int64)
}

func TestRecorder_RecordPackageMetrics(t *testing.T) {
	t.Parallel()
	creationTimestamp := time.Date(2022, 5, 27, 15, 37, 19, 0, time.UTC)

	tests := []struct {
		name                 string
		pkg                  *adapters.GenericPackage
		expectedAvailability float64
	}{
		{
			name: "unknown",
			pkg: &adapters.GenericPackage{
				Package: corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test",
						Namespace:         "test-ns",
						CreationTimestamp: metav1.Time{Time: creationTimestamp},
					},
					Status: corev1alpha1.PackageStatus{
						Revision: 32,
					},
				},
			},
			expectedAvailability: 2,
		},
		{
			name: "available",
			pkg: &adapters.GenericPackage{
				Package: corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test",
						Namespace:         "test-ns",
						CreationTimestamp: metav1.Time{Time: creationTimestamp},
					},
					Status: corev1alpha1.PackageStatus{
						Revision: 32,
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.PackageAvailable,
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			expectedAvailability: 1,
		},
		{
			name: "unavailable",
			pkg: &adapters.GenericPackage{
				Package: corev1alpha1.Package{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test",
						Namespace:         "test-ns",
						CreationTimestamp: metav1.Time{Time: creationTimestamp},
					},
					Status: corev1alpha1.PackageStatus{
						Revision: 32,
						Conditions: []metav1.Condition{
							{
								Type:   corev1alpha1.PackageAvailable,
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
			},
			expectedAvailability: 0,
		},
	}
	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := NewRecorder()
			recorder.RecordPackageMetrics(test.pkg)

			assert.Equal(t,
				test.expectedAvailability,
				testutil.ToFloat64(recorder.packageAvailability.WithLabelValues(
					test.pkg.GetName(), test.pkg.GetNamespace(),
				)))
			assert.Equal(t,
				float64(creationTimestamp.Unix()),
				testutil.ToFloat64(recorder.packageCreated.WithLabelValues(
					test.pkg.GetName(), test.pkg.GetNamespace(),
				)))
			assert.Equal(t,
				float64(32),
				testutil.ToFloat64(recorder.packageRevision.WithLabelValues(
					test.pkg.GetName(), test.pkg.GetNamespace(),
				)))
		})
	}
}

func TestRecorder_RecordPackageMetrics_delete(t *testing.T) {
	t.Parallel()
	d := metav1.Now()
	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				Namespace:         "test-ns",
				DeletionTimestamp: &d,
			},
		},
	}

	recorder := NewRecorder()
	recorder.RecordPackageMetrics(pkg)
}

func TestRecorder_RecordPackageLoadMetric(t *testing.T) {
	t.Parallel()
	pkg := &adapters.GenericPackage{
		Package: corev1alpha1.Package{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test-ns",
			},
		},
	}

	recorder := NewRecorder()
	recorder.RecordPackageLoadMetric(pkg, 10*time.Second)
	assert.Equal(t,
		float64(10),
		testutil.ToFloat64(recorder.packageLoadDuration.WithLabelValues(
			pkg.GetName(), pkg.GetNamespace(),
		)))
}

func TestRecorder_RecordObjectSetMetrics(t *testing.T) {
	t.Parallel()
	successTimestamp := time.Date(2022, 5, 27, 15, 37, 19, 0, time.UTC)
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
					Type:               corev1alpha1.ObjectSetSucceeded,
					LastTransitionTime: metav1.NewTime(successTimestamp),
					// Difference of 33 minutes and 17 seconds from `creationTimestamp`
				},
			},
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			creationTimestamp := time.Date(2022, 5, 27, 15, 4, 2, 0, time.UTC)

			obj := &unstructured.Unstructured{}
			obj.SetCreationTimestamp(metav1.NewTime(creationTimestamp))

			osMock := &genericObjectSetMock{}
			osMock.On("ClientObject").Return(obj)
			osMock.On("GetConditions").Return(&test.conditions)

			recorder := NewRecorder()
			recorder.RecordObjectSetMetrics(osMock)

			// is always emitted.
			assert.Equal(t,
				float64(creationTimestamp.Unix()),
				testutil.ToFloat64(recorder.objectSetCreated))

			if len(test.conditions) == 0 {
				assert.Equal(t, 0, testutil.CollectAndCount(recorder.objectSetSucceeded))
			} else {
				assert.Equal(t,
					float64(successTimestamp.Unix()),
					testutil.ToFloat64(recorder.objectSetSucceeded))
			}
		})
	}
}
