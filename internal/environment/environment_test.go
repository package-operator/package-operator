package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"

	"package-operator.run/internal/apis/manifests"
	hypershiftv1beta1 "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
	"package-operator.run/internal/testutil"
	"package-operator.run/internal/testutil/restmappermock"
)

// Helper function to create a context with logger for tests.
func testContextWithLogger(t *testing.T) context.Context {
	t.Helper()
	return logr.NewContext(context.Background(), testr.New(t))
}

var errExample = errors.New("boom")

type testSink struct {
	env *manifests.PackageEnvironment
}

func (s *testSink) SetEnvironment(
	env *manifests.PackageEnvironment,
) {
	s.env = env
}

func TestImplementsSinker(t *testing.T) {
	t.Parallel()
	type somethingElse struct{}

	s := &testSink{}
	res := ImplementsSinker([]any{s, &somethingElse{}})
	assert.Equal(t, []Sinker{s}, res)
}

func TestManager_Init_Kubernetes(t *testing.T) {
	t.Parallel()
	rm := &restmappermock.RestMapperMock{}
	c := testutil.NewClient()
	dc := &discoveryClientMock{}
	sink := &testSink{}

	// Mocks
	dc.
		On("ServerVersion").
		Return(&version.Info{
			GitVersion: "v1.2.3",
		}, nil)
	c.
		On(
			"Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.ClusterVersion"), mock.Anything,
		).
		Return(&meta.NoKindMatchError{})
	rm.
		On("RESTMapping", mock.Anything, mock.Anything).
		Return(&meta.RESTMapping{}, nil)

	mgr := NewManager(c, dc, rm)

	// Use context with logger instead of plain background context
	ctx := testContextWithLogger(t)
	err := mgr.Init(ctx, []Sinker{sink})
	require.NoError(t, err)

	env := sink.env
	assert.Equal(t, &manifests.PackageEnvironment{
		Kubernetes: manifests.PackageEnvironmentKubernetes{
			Version: "v1.2.3",
		},
		HyperShift: &manifests.PackageEnvironmentHyperShift{},
	}, env)
}

func TestManager_Init_OpenShift(t *testing.T) {
	t.Parallel()
	rm := &restmappermock.RestMapperMock{}
	c := testutil.NewClient()
	dc := &discoveryClientMock{}
	sink := &testSink{}

	// Mocks
	dc.
		On("ServerVersion").
		Return(&version.Info{
			GitVersion: "v1.2.3",
		}, nil)
	c.
		On(
			"Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.ClusterVersion"), mock.Anything,
		).
		Run(func(args mock.Arguments) {
			cv := args.Get(2).(*configv1.ClusterVersion)
			*cv = configv1.ClusterVersion{
				Status: configv1.ClusterVersionStatus{
					History: []configv1.UpdateHistory{
						{
							State:   configv1.CompletedUpdate,
							Version: "v123",
						},
					},
				},
			}
		}).
		Return(nil)
	c.
		On(
			"Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.Proxy"), mock.Anything,
		).
		Run(func(args mock.Arguments) {
			proxy := args.Get(2).(*configv1.Proxy)
			*proxy = configv1.Proxy{
				Status: configv1.ProxyStatus{
					HTTPProxy:  "httpxxx",
					HTTPSProxy: "httpsxxx",
					NoProxy:    "noproxyxxx",
				},
			}
		}).
		Return(nil)
	c.
		On(
			"Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.ConfigMap"), mock.Anything,
		).
		Run(func(args mock.Arguments) {
			cm := args.Get(2).(*corev1.ConfigMap)
			*cm = corev1.ConfigMap{
				Data: map[string]string{
					"test": "test123",
				},
			}
		}).
		Return(nil)
	rm.
		On("RESTMapping", mock.Anything, mock.Anything).
		Return(&meta.RESTMapping{}, nil)

	mgr := NewManager(c, dc, rm)

	// Use context with logger instead of plain background context
	ctx := testContextWithLogger(t)
	err := mgr.Init(ctx, []Sinker{sink})
	require.NoError(t, err)

	env := sink.env
	assert.Equal(t, &manifests.PackageEnvironment{
		Kubernetes: manifests.PackageEnvironmentKubernetes{
			Version: "v1.2.3",
		},
		OpenShift: &manifests.PackageEnvironmentOpenShift{
			Version: "v123",
			Managed: &manifests.PackageEnvironmentManagedOpenShift{
				Data: map[string]string{
					"test": "test123",
				},
			},
		},
		HyperShift: &manifests.PackageEnvironmentHyperShift{},
		Proxy: &manifests.PackageEnvironmentProxy{
			HTTPProxy:  "httpxxx",
			HTTPSProxy: "httpsxxx",
			NoProxy:    "noproxyxxx",
		},
	}, env)
}

func TestManager_openShiftEnvironment_API_not_registered(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()

	c.
		On(
			"Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.ClusterVersion"), mock.Anything,
		).
		Return(&meta.NoKindMatchError{})

	ctx := context.Background()
	mgr := NewManager(c, nil, nil)
	openShiftEnv, isOpenShift, err := mgr.openShiftEnvironment(ctx)
	require.NoError(t, err)
	assert.False(t, isOpenShift)
	assert.Nil(t, openShiftEnv)
}

func TestManager_openShiftEnvironment_error(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()

	c.
		On(
			"Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.ClusterVersion"), mock.Anything,
		).
		Return(errExample)

	ctx := context.Background()
	mgr := NewManager(c, nil, nil)
	_, _, err := mgr.openShiftEnvironment(ctx)
	require.ErrorIs(t, err, errExample)
}

func TestManager_openShiftProxyEnvironment_handeledErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "api not registered",
			err:  &meta.NoKindMatchError{},
		},
		{
			name: "not found",
			err:  apimachineryerrors.NewNotFound(schema.GroupResource{}, ""),
		},
	}

	for i := range tests {
		test := tests[i]

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := testutil.NewClient()

			c.
				On(
					"Get", mock.Anything, mock.Anything,
					mock.AnythingOfType("*v1.Proxy"), mock.Anything,
				).
				Return(test.err)

			ctx := context.Background()
			mgr := NewManager(c, nil, nil)
			proxyEnv, hasProxy, err := mgr.openShiftProxyEnvironment(ctx)
			require.NoError(t, err)
			assert.False(t, hasProxy)
			assert.Nil(t, proxyEnv)
		})
	}
}

func TestManager_openShiftProxyEnvironment_error(t *testing.T) {
	t.Parallel()
	c := testutil.NewClient()

	c.
		On(
			"Get", mock.Anything, mock.Anything,
			mock.AnythingOfType("*v1.Proxy"), mock.Anything,
		).
		Return(errExample)

	ctx := context.Background()
	mgr := NewManager(c, nil, nil)
	_, _, err := mgr.openShiftProxyEnvironment(ctx)
	require.ErrorIs(t, err, errExample)
}

func TestManager_hyperShiftEnvironment_handledErrors(t *testing.T) {
	t.Parallel()
	rm := &restmappermock.RestMapperMock{}

	rm.
		On(
			"RESTMapping", mock.Anything, mock.Anything,
		).
		Return(&meta.RESTMapping{}, &meta.NoResourceMatchError{})

	mgr := NewManager(nil, nil, rm)
	_, ok, err := mgr.hyperShiftEnvironment()
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestManager_hyperShiftEnvironment_error(t *testing.T) {
	t.Parallel()
	rm := &restmappermock.RestMapperMock{}

	rm.
		On(
			"RESTMapping", mock.Anything, mock.Anything,
		).
		Return(&meta.RESTMapping{}, errExample)

	mgr := NewManager(nil, nil, rm)
	_, _, err := mgr.hyperShiftEnvironment()
	require.ErrorIs(t, err, errExample)
}

type discoveryClientMock struct {
	mock.Mock
}

func (c *discoveryClientMock) ServerVersion() (*version.Info, error) {
	args := c.Called()
	return args.Get(0).(*version.Info), args.Error(1)
}

func TestSink(t *testing.T) {
	t.Parallel()
	t.Run("Kubernetes", func(t *testing.T) {
		t.Parallel()
		s := NewSink(nil)
		env := &manifests.PackageEnvironment{
			Kubernetes: manifests.PackageEnvironmentKubernetes{
				Version: "v12345",
			},
		}
		ctx := context.Background()

		s.SetEnvironment(env)
		gotEnv, err := s.GetEnvironment(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, env, gotEnv)
	})

	t.Run("OpenShift", func(t *testing.T) {
		t.Parallel()

		for _, withNodeSelector := range []bool{false, true} {
			name := "WithoutNodeSelector"
			if withNodeSelector {
				name = "WithNodeSelector"
			}

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				hc := hypershiftv1beta1.HostedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ban.ana",
						Namespace: "clusters",
						Labels: map[string]string{
							"test": "test",
						},
						Annotations: map[string]string{
							"test": "test",
						},
					},
					Spec: hypershiftv1beta1.HostedClusterSpec{},
				}

				if withNodeSelector {
					hc.Spec.NodeSelector = map[string]string{
						"apple": "pie",
					}
				}

				c := testutil.NewClient()
				c.
					On(
						"List", mock.Anything,
						mock.AnythingOfType("*v1beta1.HostedClusterList"),
						mock.Anything,
					).
					Run(func(args mock.Arguments) {
						list := args.Get(1).(*hypershiftv1beta1.HostedClusterList)
						*list = hypershiftv1beta1.HostedClusterList{
							Items: []hypershiftv1beta1.HostedCluster{hc},
						}
					}).
					Return(nil)

				s := NewSink(c)
				env := &manifests.PackageEnvironment{
					Kubernetes: manifests.PackageEnvironmentKubernetes{
						Version: "v12345",
					},
					HyperShift: &manifests.PackageEnvironmentHyperShift{},
				}
				ctx := context.Background()

				s.SetEnvironment(env)
				gotEnv, err := s.GetEnvironment(ctx, "clusters-ban-ana")
				require.NoError(t, err)

				expectedResult := &manifests.PackageEnvironment{
					Kubernetes: manifests.PackageEnvironmentKubernetes{
						Version: "v12345",
					},
					HyperShift: &manifests.PackageEnvironmentHyperShift{
						HostedCluster: &manifests.PackageEnvironmentHyperShiftHostedCluster{
							TemplateContextObjectMeta: manifests.TemplateContextObjectMeta{
								Name:        hc.Name,
								Namespace:   hc.Namespace,
								Labels:      hc.Labels,
								Annotations: hc.Annotations,
							},
							HostedClusterNamespace: "clusters-ban-ana",
						},
					},
				}

				if withNodeSelector {
					expectedResult.HyperShift.HostedCluster.NodeSelector = map[string]string{
						"apple": "pie",
					}
				}

				assert.Equal(t, expectedResult, gotEnv)
			})
		}
	})
}
