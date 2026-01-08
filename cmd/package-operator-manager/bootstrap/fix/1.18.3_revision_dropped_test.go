package fix

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/adapters"
	"package-operator.run/internal/testutil"
)

func TestRevisionDroppedFix_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMock      func(*testutil.CtrlClient)
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "returns true when ObjectSet has revision 0",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-os"},
								Spec:       corev1alpha1.ObjectSetSpec{Revision: 0},
							},
						}
					}).
					Return(nil)
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "returns true when ClusterObjectSet has revision 0",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-os"},
								Spec:       corev1alpha1.ObjectSetSpec{Revision: 1},
							},
						}
					}).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
						list.Items = []corev1alpha1.ClusterObjectSet{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-cos"},
								Spec:       corev1alpha1.ClusterObjectSetSpec{Revision: 0},
							},
						}
					}).
					Return(nil)
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "returns false when all revisions are set",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-os"},
								Spec:       corev1alpha1.ObjectSetSpec{Revision: 1},
							},
						}
					}).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
						list.Items = []corev1alpha1.ClusterObjectSet{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-cos"},
								Spec:       corev1alpha1.ClusterObjectSetSpec{Revision: 2},
							},
						}
					}).
					Return(nil)
			},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name: "returns error when ObjectSet list fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Return(errTest)
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "returns error when ClusterObjectSet list fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{}
					}).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.Anything).
					Return(errTest)
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := testutil.NewClient()
			tt.setupMock(c)

			fix := &RevisionDroppedFix{}
			result, err := fix.Check(ctx, Context{Client: c})

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
			c.AssertExpectations(t)
		})
	}
}

func TestRevisionDroppedFix_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupMock     func(*testutil.CtrlClient)
		expectedError bool
	}{
		{
			name: "successfully runs for both ObjectSets and ClusterObjectSets",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Scheme").Return(testScheme)

				// First call for ObjectSets
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{}
					}).
					Return(nil).
					Once()

				// Second call for ClusterObjectSets
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
						list.Items = []corev1alpha1.ClusterObjectSet{}
					}).
					Return(nil).
					Once()
			},
			expectedError: false,
		},
		{
			name: "returns error when ObjectSet processing fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Scheme").Return(testScheme)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Return(errTest)
			},
			expectedError: true,
		},
		{
			name: "returns error when ClusterObjectSet processing fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Scheme").Return(testScheme)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{}
					}).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.Anything).
					Return(errTest)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := logr.NewContext(context.Background(), testr.New(t))
			c := testutil.NewClient()
			tt.setupMock(c)

			fix := &RevisionDroppedFix{}
			err := fix.Run(ctx, Context{Client: c})

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			c.AssertExpectations(t)
		})
	}
}

//nolint:maintidx
func TestRevisionDroppedFix_reconcile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		objectSet       *corev1alpha1.ObjectSet
		setupMock       func(*testutil.CtrlClient, *corev1alpha1.ObjectSet)
		expectedSuccess bool
		expectedError   bool
		validateResult  func(*testing.T, *corev1alpha1.ObjectSet)
	}{
		{
			name: "returns early when revision is already set",
			objectSet: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-os",
					Namespace: "default",
				},
				Spec: corev1alpha1.ObjectSetSpec{Revision: 5},
			},
			setupMock:       func(_ *testutil.CtrlClient, _ *corev1alpha1.ObjectSet) {},
			expectedSuccess: true,
			expectedError:   false,
			validateResult: func(t *testing.T, os *corev1alpha1.ObjectSet) {
				t.Helper()
				assert.Equal(t, int64(5), os.Spec.Revision)
			},
		},
		{
			name: "sets revision to 1 when no previous revisions exist",
			objectSet: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-os",
					Namespace: "default",
				},
				Spec: corev1alpha1.ObjectSetSpec{
					Revision: 0,
					Previous: []corev1alpha1.PreviousRevisionReference{},
				},
			},
			setupMock: func(c *testutil.CtrlClient, _ *corev1alpha1.ObjectSet) {
				c.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Return(nil)
			},
			expectedSuccess: true,
			expectedError:   false,
			validateResult: func(t *testing.T, os *corev1alpha1.ObjectSet) {
				t.Helper()

				assert.Equal(t, int64(1), os.Spec.Revision)
			},
		},
		{
			name: "sets revision based on single previous revision",
			objectSet: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-os",
					Namespace: "default",
				},
				Spec: corev1alpha1.ObjectSetSpec{
					Revision: 0,
					Previous: []corev1alpha1.PreviousRevisionReference{
						{Name: "test-os-prev"},
					},
				},
			},
			setupMock: func(c *testutil.CtrlClient, _ *corev1alpha1.ObjectSet) {
				c.On("Scheme").Return(testScheme)

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						prevOS := args.Get(2).(*corev1alpha1.ObjectSet)
						prevOS.Spec.Revision = 3
					}).
					Return(nil)

				c.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Return(nil)
			},
			expectedSuccess: true,
			expectedError:   false,
			validateResult: func(t *testing.T, os *corev1alpha1.ObjectSet) {
				t.Helper()

				assert.Equal(t, int64(4), os.Spec.Revision)
			},
		},
		{
			name: "sets revision based on latest of multiple previous revisions",
			objectSet: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-os",
					Namespace: "default",
				},
				Spec: corev1alpha1.ObjectSetSpec{
					Revision: 0,
					Previous: []corev1alpha1.PreviousRevisionReference{
						{Name: "test-os-prev-1"},
						{Name: "test-os-prev-2"},
						{Name: "test-os-prev-3"},
					},
				},
			},
			setupMock: func(c *testutil.CtrlClient, _ *corev1alpha1.ObjectSet) {
				c.On("Scheme").Return(testScheme)

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev-1", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						prevOS := args.Get(2).(*corev1alpha1.ObjectSet)
						prevOS.Spec.Revision = 2
					}).
					Return(nil)

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev-2", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						prevOS := args.Get(2).(*corev1alpha1.ObjectSet)
						prevOS.Spec.Revision = 5
					}).
					Return(nil)

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev-3", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						prevOS := args.Get(2).(*corev1alpha1.ObjectSet)
						prevOS.Spec.Revision = 3
					}).
					Return(nil)

				c.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Return(nil)
			},
			expectedSuccess: true,
			expectedError:   false,
			validateResult: func(t *testing.T, os *corev1alpha1.ObjectSet) {
				t.Helper()

				assert.Equal(t, int64(6), os.Spec.Revision)
			},
		},
		{
			name: "returns false when previous revision has 0 revision (retry)",
			objectSet: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-os",
					Namespace: "default",
				},
				Spec: corev1alpha1.ObjectSetSpec{
					Revision: 0,
					Previous: []corev1alpha1.PreviousRevisionReference{
						{Name: "test-os-prev"},
					},
				},
			},
			setupMock: func(c *testutil.CtrlClient, _ *corev1alpha1.ObjectSet) {
				c.On("Scheme").Return(testScheme)

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						prevOS := args.Get(2).(*corev1alpha1.ObjectSet)
						prevOS.Spec.Revision = 0
					}).
					Return(nil)
			},
			expectedSuccess: false,
			expectedError:   false,
			validateResult: func(t *testing.T, os *corev1alpha1.ObjectSet) {
				t.Helper()

				assert.Equal(t, int64(0), os.Spec.Revision)
			},
		},
		{
			name: "returns error when getting previous revision fails",
			objectSet: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-os",
					Namespace: "default",
				},
				Spec: corev1alpha1.ObjectSetSpec{
					Revision: 0,
					Previous: []corev1alpha1.PreviousRevisionReference{
						{Name: "test-os-prev"},
					},
				},
			},
			setupMock: func(c *testutil.CtrlClient, _ *corev1alpha1.ObjectSet) {
				c.On("Scheme").Return(testScheme)

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Return(errTest)
			},
			expectedSuccess: false,
			expectedError:   true,
			validateResult:  func(_ *testing.T, _ *corev1alpha1.ObjectSet) {},
		},
		{
			name: "returns error when update fails",
			objectSet: &corev1alpha1.ObjectSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-os",
					Namespace: "default",
				},
				Spec: corev1alpha1.ObjectSetSpec{
					Revision: 0,
					Previous: []corev1alpha1.PreviousRevisionReference{
						{Name: "test-os-prev"},
					},
				},
			},
			setupMock: func(c *testutil.CtrlClient, _ *corev1alpha1.ObjectSet) {
				c.On("Scheme").Return(testScheme)

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						prevOS := args.Get(2).(*corev1alpha1.ObjectSet)
						prevOS.Spec.Revision = 2
					}).
					Return(nil)

				c.On("Update",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Return(errTest)
			},
			expectedSuccess: false,
			expectedError:   true,
			validateResult:  func(_ *testing.T, _ *corev1alpha1.ObjectSet) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := logr.NewContext(context.Background(), testr.New(t))
			c := testutil.NewClient()
			tt.setupMock(c, tt.objectSet)

			fix := &RevisionDroppedFix{}
			// Create accessor by wrapping the actual ObjectSet
			objectSetAccessor := &adapters.ObjectSetAdapter{ObjectSet: *tt.objectSet}

			success, err := fix.reconcile(ctx, objectSetAccessor, Context{Client: c}, adapters.NewObjectSet)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedSuccess, success)

			// Copy back the changes to validate
			tt.objectSet.Spec.Revision = objectSetAccessor.GetSpecRevision()
			tt.validateResult(t, tt.objectSet)

			c.AssertExpectations(t)
		})
	}
}

func TestRevisionDroppedFix_run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		setupMock        func(*testutil.CtrlClient)
		expectedFinished bool
		expectedError    bool
	}{
		{
			name: "successfully processes when no ObjectSets need fixing",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Scheme").Return(testScheme)

				// First iteration - no ObjectSets with revision 0
				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{}
					}).
					Return(nil).
					Once()
			},
			expectedFinished: true,
			expectedError:    false,
		},
		{
			name: "successfully processes ObjectSets with all revisions set",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Scheme").Return(testScheme)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-os"},
								Spec:       corev1alpha1.ObjectSetSpec{Revision: 5},
							},
						}
					}).
					Return(nil).
					Once()
			},
			expectedFinished: true,
			expectedError:    false,
		},
		{
			name: "returns not finished when ObjectSet needs fixing",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Scheme").Return(testScheme)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*corev1alpha1.ObjectSetList)
						list.Items = []corev1alpha1.ObjectSet{
							{
								ObjectMeta: metav1.ObjectMeta{Name: "test-os", Namespace: "default"},
								Spec: corev1alpha1.ObjectSetSpec{
									Revision: 0,
									Previous: []corev1alpha1.PreviousRevisionReference{
										{Name: "test-os-prev"},
									},
								},
							},
						}
					}).
					Return(nil).
					Once()

				c.On("Get",
					mock.Anything,
					client.ObjectKey{Name: "test-os-prev", Namespace: "default"},
					mock.IsType(&corev1alpha1.ObjectSet{}),
					mock.Anything).
					Run(func(args mock.Arguments) {
						prevOS := args.Get(2).(*corev1alpha1.ObjectSet)
						prevOS.Spec.Revision = 0 // Still needs fixing, so we can't proceed
					}).
					Return(nil)
			},
			expectedFinished: false,
			expectedError:    false,
		},
		{
			name: "returns error when list fails",
			setupMock: func(c *testutil.CtrlClient) {
				c.On("Scheme").Return(testScheme)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ObjectSetList{}),
					mock.Anything).
					Return(errTest)
			},
			expectedFinished: false,
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := logr.NewContext(context.Background(), testr.New(t))
			c := testutil.NewClient()
			tt.setupMock(c)

			fix := &RevisionDroppedFix{}
			finished, err := fix.run(ctx, Context{Client: c}, adapters.NewObjectSetList, adapters.NewObjectSet)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedFinished, finished)
			c.AssertExpectations(t)
		})
	}
}

func TestRevisionDroppedFix_reconcile_ClusterObjectSet(t *testing.T) {
	t.Parallel()

	t.Run("sets revision based on previous ClusterObjectSet", func(t *testing.T) {
		t.Parallel()

		ctx := logr.NewContext(context.Background(), testr.New(t))
		c := testutil.NewClient()

		c.On("Scheme").Return(testScheme)

		c.On("Get",
			mock.Anything,
			client.ObjectKey{Name: "test-cos-prev", Namespace: ""},
			mock.IsType(&corev1alpha1.ClusterObjectSet{}),
			mock.Anything).
			Run(func(args mock.Arguments) {
				prevCOS := args.Get(2).(*corev1alpha1.ClusterObjectSet)
				prevCOS.Spec.Revision = 7
			}).
			Return(nil)

		c.On("Update",
			mock.Anything,
			mock.IsType(&corev1alpha1.ClusterObjectSet{}),
			mock.Anything).
			Return(nil)

		clusterObjectSet := &corev1alpha1.ClusterObjectSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cos",
			},
			Spec: corev1alpha1.ClusterObjectSetSpec{
				Revision: 0,
				Previous: []corev1alpha1.PreviousRevisionReference{
					{Name: "test-cos-prev"},
				},
			},
		}

		fix := &RevisionDroppedFix{}
		// Create accessor by wrapping the actual ClusterObjectSet
		clusterObjectSetAccessor := &adapters.ClusterObjectSetAdapter{ClusterObjectSet: *clusterObjectSet}

		success, err := fix.reconcile(ctx, clusterObjectSetAccessor, Context{Client: c}, adapters.NewClusterObjectSet)

		require.NoError(t, err)
		assert.True(t, success)
		assert.Equal(t, int64(8), clusterObjectSetAccessor.GetSpecRevision())
		c.AssertExpectations(t)
	})
}

func TestRevisionDroppedFix_integration(t *testing.T) {
	t.Parallel()

	t.Run("Check finds ObjectSets with missing revision", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		c := testutil.NewClient()

		c.On("List",
			mock.Anything,
			mock.IsType(&corev1alpha1.ObjectSetList{}),
			mock.Anything).
			Run(func(args mock.Arguments) {
				list := args.Get(1).(*corev1alpha1.ObjectSetList)
				list.Items = []corev1alpha1.ObjectSet{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-os"},
						Spec:       corev1alpha1.ObjectSetSpec{Revision: 0},
					},
				}
			}).
			Return(nil)

		fix := &RevisionDroppedFix{}
		applicable, err := fix.Check(ctx, Context{Client: c})
		require.NoError(t, err)
		assert.True(t, applicable)
		c.AssertExpectations(t)
	})

	t.Run("Run successfully fixes ObjectSet revisions", func(t *testing.T) {
		t.Parallel()

		ctx := logr.NewContext(context.Background(), testr.New(t))
		c := testutil.NewClient()

		c.On("Scheme").Return(testScheme)

		// First run call for ObjectSets - returns empty list (nothing to fix)
		c.On("List",
			mock.Anything,
			mock.IsType(&corev1alpha1.ObjectSetList{}),
			mock.Anything).
			Run(func(args mock.Arguments) {
				list := args.Get(1).(*corev1alpha1.ObjectSetList)
				list.Items = []corev1alpha1.ObjectSet{}
			}).
			Return(nil)

		// Second run call for ClusterObjectSets - returns empty list
		c.On("List",
			mock.Anything,
			mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
			mock.Anything).
			Run(func(args mock.Arguments) {
				list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
				list.Items = []corev1alpha1.ClusterObjectSet{}
			}).
			Return(nil)

		fix := &RevisionDroppedFix{}
		err := fix.Run(ctx, Context{Client: c})
		require.NoError(t, err)
		c.AssertExpectations(t)
	})
}
