package environment

import (
	"context"
	"errors"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/package-operator/internal/testutil"
)

var errExample = errors.New("boom")

type testSink struct {
	env *manifestsv1alpha1.PackageEnvironment
}

func (s *testSink) SetEnvironment(
	env *manifestsv1alpha1.PackageEnvironment,
) {
	s.env = env
}

func TestImplementsSinker(t *testing.T) {
	t.Parallel()
	type somethingElse struct{}

	s := &testSink{}
	res := ImplementsSinker([]interface{}{s, &somethingElse{}})
	assert.Equal(t, []Sinker{s}, res)
}

func TestManager_Init_Kubernetes(t *testing.T) {
	t.Parallel()
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

	mgr := NewManager(c, dc)

	ctx := context.Background()
	err := mgr.Init(ctx, []Sinker{sink})
	require.NoError(t, err)

	env := sink.env
	assert.Equal(t, &manifestsv1alpha1.PackageEnvironment{
		Kubernetes: manifestsv1alpha1.PackageEnvironmentKubernetes{
			Version: "v1.2.3",
		},
	}, env)
}

func TestManager_Init_OpenShift(t *testing.T) {
	t.Parallel()
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

	mgr := NewManager(c, dc)

	ctx := context.Background()
	err := mgr.Init(ctx, []Sinker{sink})
	require.NoError(t, err)

	env := sink.env
	assert.Equal(t, &manifestsv1alpha1.PackageEnvironment{
		Kubernetes: manifestsv1alpha1.PackageEnvironmentKubernetes{
			Version: "v1.2.3",
		},
		OpenShift: &manifestsv1alpha1.PackageEnvironmentOpenShift{
			Version: "v123",
		},
		Proxy: &manifestsv1alpha1.PackageEnvironmentProxy{
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
	mgr := NewManager(c, nil)
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
	mgr := NewManager(c, nil)
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
			err:  k8serrors.NewNotFound(schema.GroupResource{}, ""),
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
			mgr := NewManager(c, nil)
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
	mgr := NewManager(c, nil)
	_, _, err := mgr.openShiftProxyEnvironment(ctx)
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

	s := &Sink{}
	env := &manifestsv1alpha1.PackageEnvironment{
		Kubernetes: manifestsv1alpha1.PackageEnvironmentKubernetes{
			Version: "v12345",
		},
	}
	s.SetEnvironment(env)
	gotEnv := s.GetEnvironment()
	assert.Equal(t, env, gotEnv)
}
