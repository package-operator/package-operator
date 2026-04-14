package objectsets

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"pkg.package-operator.run/boxcutter/machinery/types"
	"pkg.package-operator.run/boxcutter/managedcache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/controllers/boxcutterutil"
	"package-operator.run/internal/testutil/boxcuttermocks"
)

var errTestRemotePhaseEngine = errors.New("test error")

func TestRemoteEnabledPhaseEngineFactory_New(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMocks  func(*boxcuttermocks.PhaseEngineFactoryMock)
		expectError bool
	}{
		{
			name: "success",
			setupMocks: func(factory *boxcuttermocks.PhaseEngineFactoryMock) {
				factory.On("New", mock.Anything).
					Return(&boxcuttermocks.PhaseEngineMock{}, nil)
			},
			expectError: false,
		},
		{
			name: "factory error",
			setupMocks: func(factory *boxcuttermocks.PhaseEngineFactoryMock) {
				factory.On("New", mock.Anything).
					Return((*boxcuttermocks.PhaseEngineMock)(nil), errTestRemotePhaseEngine)
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mockFactory := &boxcuttermocks.PhaseEngineFactoryMock{}
			mockRemotePhaseReconciler := &remotePhaseReconcilerMock{}

			test.setupMocks(mockFactory)

			factory := remoteEnabledPhaseEngineFactory{
				phaseEngineFactory:    mockFactory,
				remotePhaseReconciler: mockRemotePhaseReconciler,
			}

			mockAccessor := &mockAccessor{}
			engine, err := factory.New(mockAccessor)

			if test.expectError {
				require.Error(t, err)
				assert.Nil(t, engine)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, engine)
				assert.IsType(t, &remoteEnabledPhaseEngine{}, engine)
			}

			mockFactory.AssertExpectations(t)
		})
	}
}

func TestHasClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		owner    adapters.ObjectSetAccessor
		phase    *mockPhase
		expected bool
	}{
		{
			name: "phase with class exists",
			owner: &adapters.ObjectSetAdapter{
				ObjectSet: corev1alpha1.ObjectSet{
					Spec: corev1alpha1.ObjectSetSpec{
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{
								{
									Name:  "phase1",
									Class: "remote",
								},
							},
						},
					},
				},
			},
			phase:    &mockPhase{name: "phase1"},
			expected: true,
		},
		{
			name: "phase without class",
			owner: &adapters.ObjectSetAdapter{
				ObjectSet: corev1alpha1.ObjectSet{
					Spec: corev1alpha1.ObjectSetSpec{
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{
								{
									Name: "phase1",
								},
							},
						},
					},
				},
			},
			phase:    &mockPhase{name: "phase1"},
			expected: false,
		},
		{
			name: "phase with empty class",
			owner: &adapters.ObjectSetAdapter{
				ObjectSet: corev1alpha1.ObjectSet{
					Spec: corev1alpha1.ObjectSetSpec{
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{
								{
									Name:  "phase1",
									Class: "",
								},
							},
						},
					},
				},
			},
			phase:    &mockPhase{name: "phase1"},
			expected: false,
		},
		{
			name: "phase not found in owner",
			owner: &adapters.ObjectSetAdapter{
				ObjectSet: corev1alpha1.ObjectSet{
					Spec: corev1alpha1.ObjectSetSpec{
						ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
							Phases: []corev1alpha1.ObjectSetTemplatePhase{
								{
									Name:  "phase1",
									Class: "remote",
								},
							},
						},
					},
				},
			},
			phase:    &mockPhase{name: "phase2"},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := hasClass(test.owner, test.phase)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestNewRemoteEnabledPhaseEngineFactory(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	discoveryClient := &mockDiscoveryClient{}
	restMapper := &mockRESTMapper{}
	remotePhaseReconciler := &remotePhaseReconcilerMock{}

	factory := newRemoteEnabledPhaseEngineFactory(
		scheme,
		discoveryClient,
		restMapper,
		nil, // phaseValidator can be nil for this test
		remotePhaseReconciler,
	)

	assert.NotNil(t, factory)
	assert.IsType(t, remoteEnabledPhaseEngineFactory{}, factory)

	f := factory.(remoteEnabledPhaseEngineFactory)
	assert.NotNil(t, f.phaseEngineFactory)
	assert.Equal(t, remotePhaseReconciler, f.remotePhaseReconciler)
}

func TestRemotePhaseTeardownResult(t *testing.T) {
	t.Parallel()

	t.Run("complete teardown", func(t *testing.T) {
		t.Parallel()
		result := &remotePhaseTeardownResult{
			name:        "test-phase",
			cleanupDone: true,
		}

		assert.Equal(t, "test-phase", result.GetName())
		assert.True(t, result.IsComplete())
		assert.Contains(t, result.String(), "test-phase")
		assert.Nil(t, result.Gone())
		assert.Nil(t, result.Waiting())
	})

	t.Run("incomplete teardown", func(t *testing.T) {
		t.Parallel()
		result := &remotePhaseTeardownResult{
			name:        "test-phase",
			cleanupDone: false,
		}

		assert.Equal(t, "test-phase", result.GetName())
		assert.False(t, result.IsComplete())
	})
}

func TestRemoteEnabledPhaseEngine_ReconcileRemotePhase(t *testing.T) {
	t.Parallel()

	t.Run("reconciles remote phase successfully", func(t *testing.T) {
		t.Parallel()

		mockRemotePhaseReconciler := &remotePhaseReconcilerMock{}
		engine := &remoteEnabledPhaseEngine{
			remotePhaseReconciler: mockRemotePhaseReconciler,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name:  "test-phase",
								Class: "remote",
							},
						},
					},
				},
			},
		}

		phase := &mockPhase{name: "test-phase"}
		expectedResult := &remotePhaseResult{name: "test-phase"}

		boxcutterPhase := corev1alpha1.ObjectSetTemplatePhase{
			Name:  "test-phase",
			Class: "remote",
		}

		mockRemotePhaseReconciler.On("Reconcile", mock.Anything, owner, boxcutterPhase).
			Return(expectedResult, nil)

		ctx := context.Background()
		result, err := engine.reconcileRemotePhase(ctx, owner, phase)

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockRemotePhaseReconciler.AssertExpectations(t)
	})

	t.Run("returns error from remote phase reconciler", func(t *testing.T) {
		t.Parallel()

		mockRemotePhaseReconciler := &remotePhaseReconcilerMock{}
		engine := &remoteEnabledPhaseEngine{
			remotePhaseReconciler: mockRemotePhaseReconciler,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name:  "test-phase",
								Class: "remote",
							},
						},
					},
				},
			},
		}

		phase := &mockPhase{name: "test-phase"}
		boxcutterPhase := corev1alpha1.ObjectSetTemplatePhase{
			Name:  "test-phase",
			Class: "remote",
		}

		mockRemotePhaseReconciler.On("Reconcile", mock.Anything, owner, boxcutterPhase).
			Return((*remotePhaseResult)(nil), errTestRemotePhaseEngine)

		ctx := context.Background()
		result, err := engine.reconcileRemotePhase(ctx, owner, phase)

		require.ErrorIs(t, err, errTestRemotePhaseEngine)
		assert.Nil(t, result)
		mockRemotePhaseReconciler.AssertExpectations(t)
	})
}

func TestRemoteEnabledPhaseEngine_TeardownRemotePhase(t *testing.T) {
	t.Parallel()

	t.Run("teardown completes successfully", func(t *testing.T) {
		t.Parallel()

		mockRemotePhaseReconciler := &remotePhaseReconcilerMock{}
		engine := &remoteEnabledPhaseEngine{
			remotePhaseReconciler: mockRemotePhaseReconciler,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name:  "test-phase",
								Class: "remote",
							},
						},
					},
				},
			},
		}

		phase := &mockPhase{name: "test-phase"}
		boxcutterPhase := corev1alpha1.ObjectSetTemplatePhase{
			Name:  "test-phase",
			Class: "remote",
		}

		mockRemotePhaseReconciler.On("Teardown", mock.Anything, owner, boxcutterPhase).
			Return(true, nil)

		ctx := context.Background()
		result, err := engine.teardownRemotePhase(ctx, owner, phase)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test-phase", result.GetName())
		assert.True(t, result.IsComplete())
		mockRemotePhaseReconciler.AssertExpectations(t)
	})

	t.Run("teardown in progress", func(t *testing.T) {
		t.Parallel()

		mockRemotePhaseReconciler := &remotePhaseReconcilerMock{}
		engine := &remoteEnabledPhaseEngine{
			remotePhaseReconciler: mockRemotePhaseReconciler,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name:  "test-phase",
								Class: "remote",
							},
						},
					},
				},
			},
		}

		phase := &mockPhase{name: "test-phase"}
		boxcutterPhase := corev1alpha1.ObjectSetTemplatePhase{
			Name:  "test-phase",
			Class: "remote",
		}

		mockRemotePhaseReconciler.On("Teardown", mock.Anything, owner, boxcutterPhase).
			Return(false, nil)

		ctx := context.Background()
		result, err := engine.teardownRemotePhase(ctx, owner, phase)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test-phase", result.GetName())
		assert.False(t, result.IsComplete())
		mockRemotePhaseReconciler.AssertExpectations(t)
	})

	t.Run("returns error from remote phase reconciler", func(t *testing.T) {
		t.Parallel()

		mockRemotePhaseReconciler := &remotePhaseReconcilerMock{}
		engine := &remoteEnabledPhaseEngine{
			remotePhaseReconciler: mockRemotePhaseReconciler,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name:  "test-phase",
								Class: "remote",
							},
						},
					},
				},
			},
		}

		phase := &mockPhase{name: "test-phase"}
		boxcutterPhase := corev1alpha1.ObjectSetTemplatePhase{
			Name:  "test-phase",
			Class: "remote",
		}

		mockRemotePhaseReconciler.On("Teardown", mock.Anything, owner, boxcutterPhase).
			Return(false, errTestRemotePhaseEngine)

		ctx := context.Background()
		result, err := engine.teardownRemotePhase(ctx, owner, phase)

		require.ErrorIs(t, err, errTestRemotePhaseEngine)
		assert.Nil(t, result)
		mockRemotePhaseReconciler.AssertExpectations(t)
	})
}

func TestRemoteEnabledPhaseEngine_ReconcileLocalPhase(t *testing.T) {
	t.Parallel()

	t.Run("reconciles local phase successfully", func(t *testing.T) {
		t.Parallel()

		mockPhaseEngine := &boxcuttermocks.PhaseEngineMock{}
		engine := &remoteEnabledPhaseEngine{
			pe: mockPhaseEngine,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name: "test-phase",
							},
						},
					},
				},
			},
		}

		phase := &adapters.PhaseAdapter{
			Phase: corev1alpha1.ObjectSetTemplatePhase{
				Name: "test-phase",
				Objects: []corev1alpha1.ObjectSetObject{
					{Object: unstructured.Unstructured{}},
				},
			},
			ObjectSet: owner,
		}
		ctx := context.Background()
		revision := int64(1)

		opts := []types.PhaseReconcileOption{newTestReconcileOption(owner)}

		expectedResult := &boxcuttermocks.PhaseResultMock{}
		mockPhaseEngine.On("Reconcile", ctx, revision, phase, mock.Anything).
			Return(expectedResult, nil)

		result, err := engine.Reconcile(ctx, revision, phase, opts...)

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockPhaseEngine.AssertExpectations(t)
	})

	t.Run("returns error from local phase engine", func(t *testing.T) {
		t.Parallel()

		mockPhaseEngine := &boxcuttermocks.PhaseEngineMock{}
		engine := &remoteEnabledPhaseEngine{
			pe: mockPhaseEngine,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name: "test-phase",
							},
						},
					},
				},
			},
		}

		phase := &adapters.PhaseAdapter{
			Phase: corev1alpha1.ObjectSetTemplatePhase{
				Name: "test-phase",
				Objects: []corev1alpha1.ObjectSetObject{
					{Object: unstructured.Unstructured{}},
				},
			},
			ObjectSet: owner,
		}
		ctx := context.Background()
		revision := int64(1)

		opts := []types.PhaseReconcileOption{newTestReconcileOption(owner)}

		mockPhaseEngine.On("Reconcile", ctx, revision, phase, mock.Anything).
			Return((*boxcuttermocks.PhaseResultMock)(nil), errTestRemotePhaseEngine)

		result, err := engine.Reconcile(ctx, revision, phase, opts...)

		require.ErrorIs(t, err, errTestRemotePhaseEngine)
		assert.Nil(t, result)
		mockPhaseEngine.AssertExpectations(t)
	})
}

func TestRemoteEnabledPhaseEngine_TeardownLocalPhase(t *testing.T) {
	t.Parallel()

	t.Run("teardown local phase successfully", func(t *testing.T) {
		t.Parallel()

		mockPhaseEngine := &boxcuttermocks.PhaseEngineMock{}
		engine := &remoteEnabledPhaseEngine{
			pe: mockPhaseEngine,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name: "test-phase",
							},
						},
					},
				},
			},
		}

		phase := &adapters.PhaseAdapter{
			Phase: corev1alpha1.ObjectSetTemplatePhase{
				Name: "test-phase",
				Objects: []corev1alpha1.ObjectSetObject{
					{Object: unstructured.Unstructured{}},
				},
			},
			ObjectSet: owner,
		}
		ctx := context.Background()
		revision := int64(1)

		opts := []types.PhaseTeardownOption{newTestTeardownOption(owner)}

		expectedResult := &boxcuttermocks.PhaseTeardownResultMock{}
		mockPhaseEngine.On("Teardown", ctx, revision, phase, mock.Anything).
			Return(expectedResult, nil)

		result, err := engine.Teardown(ctx, revision, phase, opts...)

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockPhaseEngine.AssertExpectations(t)
	})

	t.Run("returns error from local phase engine", func(t *testing.T) {
		t.Parallel()

		mockPhaseEngine := &boxcuttermocks.PhaseEngineMock{}
		engine := &remoteEnabledPhaseEngine{
			pe: mockPhaseEngine,
		}

		owner := &adapters.ObjectSetAdapter{
			ObjectSet: corev1alpha1.ObjectSet{
				Spec: corev1alpha1.ObjectSetSpec{
					ObjectSetTemplateSpec: corev1alpha1.ObjectSetTemplateSpec{
						Phases: []corev1alpha1.ObjectSetTemplatePhase{
							{
								Name: "test-phase",
							},
						},
					},
				},
			},
		}

		phase := &adapters.PhaseAdapter{
			Phase: corev1alpha1.ObjectSetTemplatePhase{
				Name: "test-phase",
				Objects: []corev1alpha1.ObjectSetObject{
					{Object: unstructured.Unstructured{}},
				},
			},
			ObjectSet: owner,
		}
		ctx := context.Background()
		revision := int64(1)

		opts := []types.PhaseTeardownOption{newTestTeardownOption(owner)}

		mockPhaseEngine.On("Teardown", ctx, revision, phase, mock.Anything).
			Return((*boxcuttermocks.PhaseTeardownResultMock)(nil), errTestRemotePhaseEngine)

		result, err := engine.Teardown(ctx, revision, phase, opts...)

		require.ErrorIs(t, err, errTestRemotePhaseEngine)
		assert.Nil(t, result)
		mockPhaseEngine.AssertExpectations(t)
	})
}

// Mock implementations for testing

type mockAccessor struct {
	managedcache.Accessor
}

type mockPhase struct {
	name string
}

func (m *mockPhase) GetName() string {
	return m.name
}

func (m *mockPhase) GetObjects() []client.Object {
	return nil
}

func (m *mockPhase) GetReconcileOptions() []types.PhaseReconcileOption {
	return nil
}

func (m *mockPhase) GetTeardownOptions() []types.PhaseTeardownOption {
	return nil
}

type mockDiscoveryClient struct {
	boxcutterutil.DiscoveryClient
}

type mockRESTMapper struct {
	meta.RESTMapper
}

// Test helper functions to create options with owner

type testReconcileOption struct {
	owner adapters.ObjectSetAccessor
}

func newTestReconcileOption(owner adapters.ObjectSetAccessor) types.PhaseReconcileOption {
	return &testReconcileOption{owner: owner}
}

func (o *testReconcileOption) ApplyToPhaseReconcileOptions(opts *types.PhaseReconcileOptions) {
	opts.DefaultObjectOptions = append(opts.DefaultObjectOptions, &testObjectReconcileOption{owner: o.owner})
}

func (o *testReconcileOption) ApplyToRevisionReconcileOptions(_ *types.RevisionReconcileOptions) {
	// No-op for test
}

type testObjectReconcileOption struct {
	owner adapters.ObjectSetAccessor
}

func (o *testObjectReconcileOption) ApplyToObjectReconcileOptions(opts *types.ObjectReconcileOptions) {
	switch owner := o.owner.(type) {
	case *adapters.ObjectSetAdapter:
		opts.Owner = &owner.ObjectSet
	case *adapters.ClusterObjectSetAdapter:
		opts.Owner = &owner.ClusterObjectSet
	}
}

func (o *testObjectReconcileOption) ApplyToPhaseReconcileOptions(opts *types.PhaseReconcileOptions) {
	opts.DefaultObjectOptions = append(opts.DefaultObjectOptions, o)
}

func (o *testObjectReconcileOption) ApplyToRevisionReconcileOptions(_ *types.RevisionReconcileOptions) {
	// No-op for test
}

type testTeardownOption struct {
	owner adapters.ObjectSetAccessor
}

func newTestTeardownOption(owner adapters.ObjectSetAccessor) types.PhaseTeardownOption {
	return &testTeardownOption{owner: owner}
}

func (o *testTeardownOption) ApplyToPhaseTeardownOptions(opts *types.PhaseTeardownOptions) {
	opts.DefaultObjectOptions = append(opts.DefaultObjectOptions, &testObjectTeardownOption{owner: o.owner})
}

func (o *testTeardownOption) ApplyToRevisionTeardownOptions(_ *types.RevisionTeardownOptions) {
	// No-op for test
}

type testObjectTeardownOption struct {
	owner adapters.ObjectSetAccessor
}

func (o *testObjectTeardownOption) ApplyToObjectTeardownOptions(opts *types.ObjectTeardownOptions) {
	switch owner := o.owner.(type) {
	case *adapters.ObjectSetAdapter:
		opts.Owner = &owner.ObjectSet
	case *adapters.ClusterObjectSetAdapter:
		opts.Owner = &owner.ClusterObjectSet
	}
}

func (o *testObjectTeardownOption) ApplyToPhaseTeardownOptions(opts *types.PhaseTeardownOptions) {
	opts.DefaultObjectOptions = append(opts.DefaultObjectOptions, o)
}

func (o *testObjectTeardownOption) ApplyToRevisionTeardownOptions(_ *types.RevisionTeardownOptions) {
	// No-op for test
}
