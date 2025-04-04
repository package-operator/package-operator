package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	adaptermock "package-operator.run/internal/testutil/adapters"
)

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

			assert.InDelta(t,
				test.expectedAvailability,
				testutil.ToFloat64(recorder.packageAvailability.WithLabelValues(
					test.pkg.GetName(), test.pkg.GetNamespace(), test.pkg.Spec.Image,
				)),
				0.01,
			)
			assert.InDelta(t,
				float64(creationTimestamp.Unix()),
				testutil.ToFloat64(recorder.packageCreated.WithLabelValues(
					test.pkg.GetName(), test.pkg.GetNamespace(),
				)),
				0.01,
			)
			assert.InDelta(t,
				float64(32),
				testutil.ToFloat64(recorder.packageRevision.WithLabelValues(
					test.pkg.GetName(), test.pkg.GetNamespace(),
				)),
				0.01,
			)
		})
	}
}

// Ensures that only a single package availability timeseries is exposed after image changes.
func TestRecorder_RecordPackageMetrics_updateImage(t *testing.T) {
	t.Parallel()

	newPackage := func(image string) *adapters.GenericPackage {
		return &adapters.GenericPackage{
			Package: corev1alpha1.Package{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test",
					Namespace:         "test-ns",
					CreationTimestamp: metav1.Time{Time: time.Date(2022, 5, 27, 15, 37, 19, 0, time.UTC)},
				},
				Spec: corev1alpha1.PackageSpec{
					Image: image,
				},
			},
		}
	}

	recorder := NewRecorder()

	for _, image := range []string{
		"image:a",
		"image:b",
		"image:c",
	} {
		pkg := newPackage(image)
		recorder.RecordPackageMetrics(pkg)

		// Assert that only a single timeseries is present. (Timeseries with a previous image label must be dropped.)
		assert.Equal(t, 1, testutil.CollectAndCount(recorder.packageAvailability))
		// Assert that the present timeseries is using the correct up-to-date label.
		assert.Equal(t, 1, testutil.CollectAndCount(
			recorder.packageAvailability.WithLabelValues(pkg.Name, pkg.Namespace, pkg.Spec.Image)))
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
	assert.InDelta(t,
		float64(10),
		testutil.ToFloat64(recorder.packageLoadDuration.WithLabelValues(
			pkg.GetName(), pkg.GetNamespace(),
		)),
		0.01,
	)
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

			osMock := &adaptermock.ObjectSetMock{}
			osMock.On("ClientObject").Return(obj)
			osMock.On("GetConditions").Return(&test.conditions)

			recorder := NewRecorder()
			recorder.RecordObjectSetMetrics(osMock)

			// is always emitted.
			assert.InDelta(t,
				float64(creationTimestamp.Unix()),
				testutil.ToFloat64(recorder.objectSetCreated),
				0.01,
			)

			if len(test.conditions) == 0 {
				assert.Equal(t, 0, testutil.CollectAndCount(recorder.objectSetSucceeded))
			} else {
				assert.InDelta(t,
					float64(successTimestamp.Unix()),
					testutil.ToFloat64(recorder.objectSetSucceeded),
					0.01,
				)
			}
		})
	}
}
