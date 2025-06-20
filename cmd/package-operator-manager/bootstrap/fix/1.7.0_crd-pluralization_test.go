package fix

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

func init() {
	deletionWaitInterval = time.Millisecond
	deletionWaitTimeout = 10 * time.Millisecond
}

var errTest = errors.New("test")

func TestMustParseLabelSelector(t *testing.T) {
	t.Parallel()

	t.Run("Parsing correct selector string must succeed.", func(t *testing.T) {
		t.Parallel()

		require.NotPanics(t, func() {
			mustParseLabelSelector("app.kubernetes.io/name=foo")
		})
	})

	t.Run("Parsing incorrect selector string must fail.", func(t *testing.T) {
		t.Parallel()

		require.Panics(t, func() {
			mustParseLabelSelector("f.,<>bla")
		})
	})

	t.Run("Parsing `pkoClusterObjectSetLabelSelector` must not fail.", func(t *testing.T) {
		t.Parallel()

		require.NotPanics(t, func() {
			require.NotNil(t,
				mustParseLabelSelector(pkoClusterObjectSetLabelSelector))
		})
	})
}

const labelSelectorString = "test=true"

type (
	subTestFunc func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, scheme *runtime.Scheme)
	subTest     struct {
		name string
		t    subTestFunc
	}
)

func TestCRDPluralizationFix_ensureClusterObjectSetsGoneWithOrphansLeft(t *testing.T) {
	t.Parallel()

	subTests := []subTest{
		{
			name: "happyPath",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				c.On("Scheme").Return(testScheme)

				c.On("DeleteAllOf",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.IsType([]client.DeleteAllOfOption{})).
					Run(func(args mock.Arguments) {
						// Test if delete options contain the label selector and orphaning deletion!
						opts := args.Get(2).([]client.DeleteAllOfOption)
						assert.Contains(t, opts, client.PropagationPolicy(metav1.DeletePropagationOrphan))

						assert.Condition(t, hasDeleteAllOfOption(opts, client.MatchingLabelsSelector{
							Selector: mustParseLabelSelector(labelSelectorString),
						}))
					}).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.IsType([]client.ListOption{})).
					Run(func(args mock.Arguments) {
						// mock single ClusterObjectSet with a finalizer
						list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
						list.Items = append(list.Items, corev1alpha1.ClusterObjectSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:       "cos",
								Finalizers: []string{"finalizer"},
							},
						})

						// Test if list options contain the label selector
						opts := args.Get(2).([]client.ListOption)
						assert.Condition(t, hasListOption(opts, client.MatchingLabelsSelector{
							Selector: mustParseLabelSelector(labelSelectorString),
						}))
					}).
					Return(nil)

				c.On("Patch",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.Anything,
					mock.Anything).
					Run(func(args mock.Arguments) {
						// Test if finalizers are empty.
						cos := args.Get(1).(*corev1alpha1.ClusterObjectSet)
						assert.Equal(t, []string{}, cos.ObjectMeta.Finalizers)
					}).
					Return(nil)

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.Anything).
					Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				fix := &CRDPluralizationFix{}
				require.NoError(t, fix.ensureClusterObjectSetsGoneWithOrphansLeft(ctx, Context{
					Client: c,
					Log:    log,
				}, labelSelectorString))
			},
		},
		{
			name: "deleteAllOfFails",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				testErr := errors.New("test")

				c.On("DeleteAllOf",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.IsType([]client.DeleteAllOfOption{})).
					Return(testErr)

				fix := &CRDPluralizationFix{}
				require.ErrorIs(t, fix.ensureClusterObjectSetsGoneWithOrphansLeft(ctx, Context{
					Client: c,
					Log:    log,
				}, labelSelectorString), testErr)
			},
		},
		{
			name: "listFails",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				c.On("DeleteAllOf",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.IsType([]client.DeleteAllOfOption{})).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.IsType([]client.ListOption{})).
					Return(errTest)

				fix := &CRDPluralizationFix{}
				require.ErrorIs(t, fix.ensureClusterObjectSetsGoneWithOrphansLeft(ctx, Context{
					Client: c,
					Log:    log,
				}, labelSelectorString), errTest)
			},
		},
		{
			name: "patchFails",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				testErr := errors.New("test")

				c.On("DeleteAllOf",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.IsType([]client.DeleteAllOfOption{})).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.IsType([]client.ListOption{})).
					Run(func(args mock.Arguments) {
						// mock single ClusterObjectSet with a finalizer
						list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
						list.Items = append(list.Items, corev1alpha1.ClusterObjectSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:       "cos",
								Finalizers: []string{"finalizer"},
							},
						})
					}).
					Return(nil)

				c.On("Patch",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.Anything,
					mock.Anything).
					Return(testErr)

				fix := &CRDPluralizationFix{}
				require.ErrorIs(t, fix.ensureClusterObjectSetsGoneWithOrphansLeft(ctx, Context{
					Client: c,
					Log:    log,
				}, labelSelectorString), testErr)
			},
		},
		{
			name: "waitToBeGoneFails",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				testErr := errors.New("test")

				c.On("Scheme").Return(testScheme)

				c.On("DeleteAllOf",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.IsType([]client.DeleteAllOfOption{})).
					Return(nil)

				c.On("List",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSetList{}),
					mock.IsType([]client.ListOption{})).
					Run(func(args mock.Arguments) {
						// mock single ClusterObjectSet with a finalizer
						list := args.Get(1).(*corev1alpha1.ClusterObjectSetList)
						list.Items = append(list.Items, corev1alpha1.ClusterObjectSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:       "cos",
								Finalizers: []string{"finalizer"},
							},
						})
					}).
					Return(nil)

				c.On("Patch",
					mock.Anything,
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.Anything,
					mock.Anything).
					Return(nil)

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&corev1alpha1.ClusterObjectSet{}),
					mock.Anything).
					Return(testErr)

				fix := &CRDPluralizationFix{}
				require.ErrorContains(t,
					fix.ensureClusterObjectSetsGoneWithOrphansLeft(ctx, Context{Client: c, Log: log}, labelSelectorString),
					"timeout waiting 10ms for package-operator.run/v1alpha1, Kind=ClusterObjectSet /cos to be gone",
				)
			},
		},
	}

	for _, subTest := range subTests {
		t.Run(subTest.name, func(t *testing.T) {
			t.Parallel()

			ctx := logr.NewContext(context.Background(), testr.New(t))
			c := testutil.NewClient()

			subTest.t(t, c, ctx, testScheme)
			c.AssertExpectations(t)
		})
	}
}

func TestEnsureCRDGone(t *testing.T) {
	t.Parallel()

	subTests := []subTest{
		{
			name: "happyPathAlreadyGone",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				c.On("Delete",
					mock.Anything,
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				fix := &CRDPluralizationFix{}
				require.NoError(t, fix.ensureCRDGone(ctx, Context{
					Client: c,
					Log:    log,
				}, "foo"))
			},
		},
		{
			name: "happyPath",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				c.On("Scheme").Return(testScheme)

				c.On("Delete",
					mock.Anything,
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(nil)
				c.On("Get",
					mock.Anything,
					mock.Anything,
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(apimachineryerrors.NewNotFound(schema.GroupResource{}, ""))

				fix := &CRDPluralizationFix{}
				require.NoError(t, fix.ensureCRDGone(ctx, Context{
					Client: c,
					Log:    log,
				}, "foo"))
			},
		},
		{
			name: "deleteFails",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				c.On("Delete",
					mock.Anything,
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(errTest)

				fix := &CRDPluralizationFix{}
				require.ErrorIs(t, fix.ensureCRDGone(ctx, Context{
					Client: c,
					Log:    log,
				}, "foo"), errTest)
			},
		},
		{
			name: "waitFails",
			t: func(t *testing.T, c *testutil.CtrlClient, ctx context.Context, _ *runtime.Scheme) {
				t.Helper()

				log, err := logr.FromContext(ctx)
				require.NoError(t, err)

				c.On("Scheme").Return(testScheme)

				c.On("Delete",
					mock.Anything,
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(nil)

				c.On("Get",
					mock.Anything,
					mock.IsType(client.ObjectKey{}),
					mock.IsType(&apiextensionsv1.CustomResourceDefinition{}),
					mock.Anything).
					Return(errTest)

				fix := &CRDPluralizationFix{}
				require.ErrorContains(t,
					fix.ensureCRDGone(ctx, Context{Client: c, Log: log}, "foo"),
					"timeout waiting 10ms for apiextensions.k8s.io/v1, Kind=CustomResourceDefinition /foo to be gone",
				)
			},
		},
	}

	for _, subTest := range subTests {
		t.Run(subTest.name, func(t *testing.T) {
			t.Parallel()
			t.Helper()

			ctx := logr.NewContext(context.Background(), testr.New(t))
			c := testutil.NewClient()

			subTest.t(t, c, ctx, testScheme)
			c.AssertExpectations(t)
		})
	}
}

func hasDeleteAllOfOption(opts []client.DeleteAllOfOption, expected client.DeleteAllOfOption) func() bool {
	return func() bool {
		for _, opt := range opts {
			if equality.Semantic.DeepEqual(opt, expected) {
				return true
			}
		}
		return false
	}
}

func hasListOption(opts []client.ListOption, expected client.ListOption) func() bool {
	return func() bool {
		for _, opt := range opts {
			if equality.Semantic.DeepEqual(opt, expected) {
				return true
			}
		}
		return false
	}
}
