package objectsets

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/package-operator/internal/controllers"
	"package-operator.run/package-operator/internal/testutil/controllersmocks"
)

type phaseReconcilerMock = controllersmocks.PhaseReconcilerMock

type remotePhaseReconcilerMock struct {
	mock.Mock
}

func (m *remotePhaseReconcilerMock) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) ([]corev1alpha1.ControlledObjectReference, controllers.ProbingResult, error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Get(0).([]corev1alpha1.ControlledObjectReference),
		args.Get(1).(controllers.ProbingResult),
		args.Error(2)
}

func (m *remotePhaseReconcilerMock) Teardown(
	ctx context.Context, objectSet genericObjectSet,
	phase corev1alpha1.ObjectSetTemplatePhase,
) (cleanupDone bool, err error) {
	args := m.Called(ctx, objectSet, phase)
	return args.Bool(0), args.Error(1)
}

func TestObjectSetPhasesReconciler_Reconcile(t *testing.T) {
	t.Parallel()

	pr := &phaseReconcilerMock{}
	remotePr := &remotePhaseReconcilerMock{}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{}, nil
	}
	r := newObjectSetPhasesReconciler(testScheme, pr, remotePr, lookup)

	phase1 := corev1alpha1.ObjectSetTemplatePhase{
		Name: "phase1",
	}
	phase2 := corev1alpha1.ObjectSetTemplatePhase{
		Name:  "phase2",
		Class: "class",
	}

	os := &GenericObjectSet{}
	os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
		phase1,
		phase2,
	}

	pr.On("ReconcilePhase", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]client.Object{}, controllers.ProbingResult{}, nil)
	remotePr.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
		Return([]corev1alpha1.ControlledObjectReference{}, controllers.ProbingResult{}, nil)

	res, err := r.Reconcile(context.Background(), os)
	assert.Empty(t, res)
	assert.NoError(t, err)

	pr.AssertCalled(t, "ReconcilePhase", mock.Anything, mock.Anything, phase1, mock.Anything, mock.Anything)
	remotePr.AssertCalled(t, "Reconcile", mock.Anything, mock.Anything, phase2)

	conds := *os.GetConditions()
	require.Len(t, conds, 2)
	var succeededCond, availableCond metav1.Condition
	for _, cond := range conds {
		if cond.Type == corev1alpha1.ObjectSetSucceeded {
			succeededCond = cond
		} else if cond.Type == corev1alpha1.ObjectSetAvailable {
			availableCond = cond
		}
	}
	assert.Equal(t, metav1.ConditionTrue, succeededCond.Status)
	assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
}

func TestPhaseReconciler_ReconcileBackoff(t *testing.T) {
	t.Parallel()

	pr := &phaseReconcilerMock{}
	remotePr := &remotePhaseReconcilerMock{}
	lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
		return []controllers.PreviousObjectSet{}, nil
	}
	r := newObjectSetPhasesReconciler(testScheme, pr, remotePr, lookup)

	os := &GenericObjectSet{}
	os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
		{
			Name: "phase1",
		},
	}

	pr.On("ReconcilePhase", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]client.Object{}, controllers.ProbingResult{}, controllers.NewExternalResourceNotFoundError(nil))
	remotePr.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
		Return([]corev1alpha1.ControlledObjectReference{}, controllers.ProbingResult{}, nil)

	res, err := r.Reconcile(context.Background(), os)
	require.NoError(t, err)

	assert.Equal(t, reconcile.Result{
		RequeueAfter: controllers.DefaultInitialBackoff,
	}, res)
}

func TestObjectSetPhasesReconciler_Teardown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		firstTeardownFinish bool
	}{
		{
			"confirm phase2 torndown first",
			false,
		},
		{
			"all teardowns finish",
			true,
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pr := &phaseReconcilerMock{}
			remotePr := &remotePhaseReconcilerMock{}
			lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
				return []controllers.PreviousObjectSet{}, nil
			}
			r := newObjectSetPhasesReconciler(testScheme, pr, remotePr, lookup)

			phase1 := corev1alpha1.ObjectSetTemplatePhase{
				Name: "phase1",
			}
			phase2 := corev1alpha1.ObjectSetTemplatePhase{
				Name:  "phase2",
				Class: "class",
			}

			os := &GenericObjectSet{}
			os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
				phase1,
				phase2,
			}
			remotePr.On("Teardown", mock.Anything, mock.Anything, mock.Anything).
				Return(test.firstTeardownFinish, nil).Once()
			pr.On("TeardownPhase", mock.Anything, mock.Anything, mock.Anything).
				Return(true, nil).Maybe()

			done, err := r.Teardown(context.Background(), os)
			assert.Equal(t, test.firstTeardownFinish, done)
			assert.NoError(t, err)
			remotePr.AssertCalled(t, "Teardown", mock.Anything, mock.Anything, phase2)
			if test.firstTeardownFinish {
				pr.AssertCalled(t, "TeardownPhase", mock.Anything, mock.Anything, phase1)
			}
		})
	}
}

func TestObjectSetPhasesReconciler_SuccessDelay(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ObjectSet                 genericObjectSet
		TimeSinceAvailable        time.Duration
		ExpectedConditionStatuses map[string]metav1.ConditionStatus
	}{
		"success delay default": {
			ObjectSet: &GenericObjectSet{
				ObjectSet: corev1alpha1.ObjectSet{
					Spec: corev1alpha1.ObjectSetSpec{
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{
								{
									Name: "phase-1",
								},
							},
						},
					},
				},
			},
			ExpectedConditionStatuses: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectSetAvailable: metav1.ConditionTrue,
				corev1alpha1.ObjectSetSucceeded: metav1.ConditionTrue,
			},
		},
		"success delay 2s/time since available 1s": {
			ObjectSet: &GenericObjectSet{
				ObjectSet: corev1alpha1.ObjectSet{
					Spec: corev1alpha1.ObjectSetSpec{
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{
								{
									Name: "phase-1",
								},
							},
							SuccessDelaySeconds: 2,
						},
					},
				},
			},
			TimeSinceAvailable: 1 * time.Second,
			ExpectedConditionStatuses: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectSetAvailable: metav1.ConditionTrue,
			},
		},
		"success delay 1s/time since available 2s": {
			ObjectSet: &GenericObjectSet{
				ObjectSet: corev1alpha1.ObjectSet{
					Spec: corev1alpha1.ObjectSetSpec{
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{
								{
									Name: "phase-1",
								},
							},
							SuccessDelaySeconds: 1,
						},
					},
				},
			},
			TimeSinceAvailable: 2 * time.Second,
			ExpectedConditionStatuses: map[string]metav1.ConditionStatus{
				corev1alpha1.ObjectSetAvailable: metav1.ConditionTrue,
				corev1alpha1.ObjectSetSucceeded: metav1.ConditionTrue,
			},
		},
	}

	for i := range tests {
		name := i
		tc := tests[i]
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cm := &clockMock{}
			prm := &phaseReconcilerMock{}
			rprm := &remotePhaseReconcilerMock{}
			lookup := func(_ context.Context, _ controllers.PreviousOwner) ([]controllers.PreviousObjectSet, error) {
				return []controllers.PreviousObjectSet{}, nil
			}

			cm.On("Now").Return(time.Now().Add(tc.TimeSinceAvailable))
			prm.On("ReconcilePhase", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return([]client.Object{}, controllers.ProbingResult{}, nil)
			rprm.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
				Return([]corev1alpha1.ControlledObjectReference{}, controllers.ProbingResult{}, nil)

			rec := newObjectSetPhasesReconciler(
				testScheme, prm, rprm, lookup,
				withClock{
					Clock: cm,
				},
			)
			_, err := rec.Reconcile(context.Background(), tc.ObjectSet)
			require.NoError(t, err)

			require.Equal(t, len(tc.ExpectedConditionStatuses), len(*tc.ObjectSet.GetConditions()), tc.ObjectSet.GetConditions())

			for cond, stat := range tc.ExpectedConditionStatuses {
				require.True(t, meta.IsStatusConditionPresentAndEqual(*tc.ObjectSet.GetConditions(), cond, stat))
			}
		})
	}
}

type clockMock struct {
	mock.Mock
}

func (m *clockMock) Now() time.Time {
	args := m.Called()

	return args.Get(0).(time.Time)
}

func Test_isObjectSetInTransition(t *testing.T) {
	t.Parallel()

	examplePod := unstructured.Unstructured{}
	examplePod.SetGroupVersionKind(schema.GroupVersionKind{
		Kind: "Pod", Version: "v1",
	})
	examplePod.SetName("pod-1")

	testObjectSet1 := newGenericObjectSet(testScheme)
	testObjectSet1.ClientObject().SetNamespace("test-ns")
	testObjectSet1.SetPhases([]corev1alpha1.ObjectSetTemplatePhase{
		{
			Name: "a",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: examplePod,
				},
			},
		},
	})

	testObjectSetArchived := &GenericObjectSet{
		ObjectSet: *testObjectSet1.ClientObject().DeepCopyObject().(*corev1alpha1.ObjectSet),
	}
	testObjectSetArchived.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived

	tests := []struct {
		name         string
		objectSet    genericObjectSet
		controllerOf []corev1alpha1.ControlledObjectReference
		expected     bool
	}{
		{
			name:      "empty",
			objectSet: newGenericObjectSet(testScheme),
			expected:  false,
		},
		{
			name:      "pod in transition",
			objectSet: testObjectSet1,
			expected:  true,
		},
		{
			name:      "pod not in transition",
			objectSet: testObjectSet1,
			controllerOf: []corev1alpha1.ControlledObjectReference{
				{Kind: "Pod", Name: "pod-1", Namespace: testObjectSet1.ClientObject().GetNamespace()},
			},
			expected: false,
		},
		{
			// clone of "pod in transition" test, but with archived lifecycle state.
			name:      "archived is never in transition",
			objectSet: testObjectSetArchived,
			expected:  false,
		},
		{
			name:      "pod not in transition with cluster scope",
			objectSet: testObjectSet1,
			controllerOf: []corev1alpha1.ControlledObjectReference{
				{Kind: "Pod", Name: "pod-1"},
			},
			expected: false,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := isObjectSetInTransition(test.objectSet, test.controllerOf)
			assert.Equal(t, test.expected, r)
		})
	}
}
