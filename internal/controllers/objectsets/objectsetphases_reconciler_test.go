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

	"pkg.package-operator.run/boxcutter/machinery"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/preflight"
	"package-operator.run/internal/testutil/boxcuttermocks"
	"package-operator.run/internal/testutil/managedcachemocks"
)

const testUID = "test-UID"

func TestObjectSetPhasesReconciler_Reconcile(t *testing.T) {
	t.Parallel()

	type prepared struct {
		accessManager             *managedcachemocks.ObjectBoundAccessManagerMock[client.Object]
		accessor                  *managedcachemocks.AccessorMock
		factory                   *boxcuttermocks.RevisionEngineFactoryMock
		checker                   *phasesCheckerMock
		revisionEngine            *boxcuttermocks.RevisionEngineMock
		remotePhaseReconciler     *remotePhaseReconcilerMock
		objectSetPhasesReconciler *objectSetPhasesReconciler
	}

	prepare := func() *prepared {
		accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
		accessor := &managedcachemocks.AccessorMock{}
		factory := &boxcuttermocks.RevisionEngineFactoryMock{}
		revisionEngine := &boxcuttermocks.RevisionEngineMock{}
		remotePhaseReconciler := &remotePhaseReconcilerMock{}

		accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(accessor, nil)
		factory.On("New", accessor).Return(revisionEngine, nil)

		checker := &phasesCheckerMock{}

		lookup := func(_ context.Context, _ controllers.PreviousOwner) (
			[]controllers.PreviousObjectSet,
			error,
		) {
			return []controllers.PreviousObjectSet{}, nil
		}

		objectSetPhasesReconciler := newObjectSetPhasesReconciler(
			testScheme,
			accessManager,
			factory,
			remotePhaseReconciler,
			lookup,
			checker,
		)

		return &prepared{
			accessManager,
			accessor,
			factory,
			checker,
			revisionEngine,
			remotePhaseReconciler,
			objectSetPhasesReconciler,
		}
	}

	t.Run("Reconcile", func(t *testing.T) {
		t.Parallel()

		p := prepare()

		phase1 := corev1alpha1.ObjectSetTemplatePhase{
			Name: "phase1",
		}
		phase2 := corev1alpha1.ObjectSetTemplatePhase{
			Name:  "phase2",
			Class: "class",
		}

		os := &adapters.ObjectSetAdapter{}
		os.UID = testUID
		os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
			phase1,
			phase2,
		}

		// Mock the revision result
		revisionResult := &boxcuttermocks.RevisionResultMock{}
		revisionResult.On("GetPhases").Return([]machinery.PhaseResult{})
		revisionResult.On("IsComplete").Return(true)

		p.revisionEngine.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
			Return(revisionResult, nil)
		p.checker.On("Check", mock.Anything, mock.Anything).Return([]preflight.Violation{}, nil)

		res, err := p.objectSetPhasesReconciler.Reconcile(context.Background(), os)
		assert.Empty(t, res)
		require.NoError(t, err)

		p.revisionEngine.AssertExpectations(t)
		p.checker.AssertExpectations(t)
		revisionResult.AssertExpectations(t)

		conds := *os.GetStatusConditions()
		require.Len(t, conds, 2)
		var succeededCond, availableCond metav1.Condition
		for _, cond := range conds {
			switch cond.Type {
			case corev1alpha1.ObjectSetSucceeded:
				succeededCond = cond
			case corev1alpha1.ObjectSetAvailable:
				availableCond = cond
			}
		}
		assert.Equal(t, metav1.ConditionTrue, succeededCond.Status)
		assert.Equal(t, metav1.ConditionTrue, availableCond.Status)
	})

	t.Run("ReconcileBackoff", func(t *testing.T) {
		t.Parallel()

		p := prepare()

		os := &adapters.ObjectSetAdapter{}
		os.UID = testUID
		os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
			{
				Name: "phase1",
			},
		}

		p.revisionEngine.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
			Return((*boxcuttermocks.RevisionResultMock)(nil), controllers.NewExternalResourceNotFoundError(nil))
		p.checker.On("Check", mock.Anything, mock.Anything).Return([]preflight.Violation{}, nil)

		res, err := p.objectSetPhasesReconciler.Reconcile(context.Background(), os)
		require.NoError(t, err)

		p.checker.AssertCalled(t, "Check", mock.Anything, mock.Anything)
		assert.Equal(t, reconcile.Result{
			RequeueAfter: controllers.DefaultInitialBackoff,
		}, res)
	})

	t.Run("Teardown", func(t *testing.T) {
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

				p := prepare()

				phase1 := corev1alpha1.ObjectSetTemplatePhase{
					Name: "phase1",
				}
				phase2 := corev1alpha1.ObjectSetTemplatePhase{
					Name:  "phase2",
					Class: "class",
				}

				os := &adapters.ObjectSetAdapter{}
				os.UID = testUID // Required for WithOwner validation
				os.Spec.Phases = []corev1alpha1.ObjectSetTemplatePhase{
					phase1,
					phase2,
				}

				// Mock the teardown result
				teardownResult := &boxcuttermocks.RevisionTeardownResultMock{}
				teardownResult.On("IsComplete").Return(test.firstTeardownFinish)

				p.revisionEngine.On("Teardown", mock.Anything, mock.Anything, mock.Anything).
					Return(teardownResult, nil)
				p.accessManager.On("FreeWithUser", mock.Anything, mock.Anything, mock.Anything).Return(nil)

				done, err := p.objectSetPhasesReconciler.Teardown(context.Background(), os)
				assert.Equal(t, test.firstTeardownFinish, done)
				require.NoError(t, err)
				p.revisionEngine.AssertExpectations(t)
				teardownResult.AssertExpectations(t)
				if test.firstTeardownFinish {
					p.accessManager.AssertCalled(t, "FreeWithUser", mock.Anything, mock.Anything, mock.Anything)
				} else {
					p.accessManager.AssertNotCalled(t, "FreeWithUser", mock.Anything, mock.Anything, os)
				}
			})
		}
	})
}

func TestObjectSetPhasesReconciler_SuccessDelay(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ObjectSet                 adapters.ObjectSetAccessor
		TimeSinceAvailable        time.Duration
		ExpectedConditionStatuses map[string]metav1.ConditionStatus
	}{
		"success delay default": {
			ObjectSet: &adapters.ObjectSetAdapter{
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
			ObjectSet: &adapters.ObjectSetAdapter{
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
			ObjectSet: &adapters.ObjectSetAdapter{
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

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			accessManager := &managedcachemocks.ObjectBoundAccessManagerMock[client.Object]{}
			accessor := &managedcachemocks.AccessorMock{}
			factory := &boxcuttermocks.RevisionEngineFactoryMock{}
			revisionEngine := &boxcuttermocks.RevisionEngineMock{}
			remotePhaseReconciler := &remotePhaseReconcilerMock{}

			accessManager.On("GetWithUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(accessor, nil)
			factory.On("New", accessor).Return(revisionEngine, nil)

			lookup := func(_ context.Context, _ controllers.PreviousOwner) (
				[]controllers.PreviousObjectSet,
				error,
			) {
				return []controllers.PreviousObjectSet{}, nil
			}
			checker := &phasesCheckerMock{}

			clock := &clockMock{}

			// Mock the revision result
			revisionResult := &boxcuttermocks.RevisionResultMock{}
			revisionResult.On("GetPhases").Return([]machinery.PhaseResult{})
			revisionResult.On("IsComplete").Return(true)

			clock.On("Now").Return(time.Now().Add(tc.TimeSinceAvailable))
			revisionEngine.On("Reconcile", mock.Anything, mock.Anything, mock.Anything).
				Return(revisionResult, nil)
			checker.On("Check", mock.Anything, mock.Anything).Return([]preflight.Violation{}, nil)

			rec := newObjectSetPhasesReconciler(
				testScheme,
				accessManager,
				factory,
				remotePhaseReconciler,
				lookup,
				checker,
				withClock{
					Clock: clock,
				},
			)
			tc.ObjectSet.ClientObject().SetUID(testUID) // Required for WithOwner validation
			_, err := rec.Reconcile(context.Background(), tc.ObjectSet)
			require.NoError(t, err)

			require.Len(t,
				*tc.ObjectSet.GetStatusConditions(), len(tc.ExpectedConditionStatuses),
				tc.ObjectSet.GetStatusConditions(),
			)
			checker.AssertCalled(t, "Check", mock.Anything, mock.Anything)

			for cond, stat := range tc.ExpectedConditionStatuses {
				require.True(t, meta.IsStatusConditionPresentAndEqual(*tc.ObjectSet.GetStatusConditions(), cond, stat))
			}
		})
	}
}

func Test_isObjectSetInTransition(t *testing.T) {
	t.Parallel()

	examplePod := unstructured.Unstructured{}
	examplePod.SetGroupVersionKind(schema.GroupVersionKind{
		Kind: "Pod", Version: "v1",
	})
	examplePod.SetName("pod-1")

	testObjectSet1 := adapters.NewObjectSet(testScheme)
	testObjectSet1.ClientObject().SetNamespace("test-ns")
	testObjectSet1.SetSpecPhases([]corev1alpha1.ObjectSetTemplatePhase{
		{
			Name: "a",
			Objects: []corev1alpha1.ObjectSetObject{
				{
					Object: examplePod,
				},
			},
		},
	})

	testObjectSetArchived := &adapters.ObjectSetAdapter{
		ObjectSet: *testObjectSet1.ClientObject().DeepCopyObject().(*corev1alpha1.ObjectSet),
	}
	testObjectSetArchived.Spec.LifecycleState = corev1alpha1.ObjectSetLifecycleStateArchived

	tests := []struct {
		name         string
		objectSet    adapters.ObjectSetAccessor
		controllerOf []corev1alpha1.ControlledObjectReference
		expected     bool
	}{
		{
			name:      "empty",
			objectSet: adapters.NewObjectSet(testScheme),
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

// Mock types for testing

type withClock struct {
	Clock clock
}

func (w withClock) ConfigureObjectSetPhasesReconciler(cfg *objectSetPhasesReconcilerConfig) {
	cfg.Clock = w.Clock
}
