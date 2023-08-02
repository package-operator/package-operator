package bootstrap

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/testutil"
)

func TestBootstrapperBootstrap(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	var initCalled bool
	var fixCalled bool
	b := &Bootstrapper{
		log:    testr.New(t),
		client: c,
		init: func(ctx context.Context) (
			bool, error,
		) {
			initCalled = true
			return true, nil
		},
		fix: func(ctx context.Context) error {
			fixCalled = true
			return nil
		},
	}
	b.SetEnvironment(&manifestsv1alpha1.PackageEnvironment{
		Proxy: &manifestsv1alpha1.PackageEnvironmentProxy{
			HTTPProxy:  "httpxxx",
			HTTPSProxy: "httpsxxx",
			NoProxy:    "noxxx",
		},
	})

	c.On("Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1.Deployment"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			depl := args.Get(2).(*appsv1.Deployment)
			depl.Status.AvailableReplicas = 1
		}).
		Return(nil)

	ctx := context.Background()
	err := b.Bootstrap(
		ctx, func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
	require.NoError(t, err)
	assert.True(t, initCalled)
	assert.True(t, fixCalled)

	assert.Equal(t, os.Getenv("HTTP_PROXY"), "httpxxx")
	assert.Equal(t, os.Getenv("HTTPS_PROXY"), "httpsxxx")
	assert.Equal(t, os.Getenv("NO_PROXY"), "noxxx")
}

func TestBootstrapper_bootstrap(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	b := &Bootstrapper{client: c}

	var (
		runManagerCalled bool
		runManagerCtx    context.Context
	)

	c.On("Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1.Deployment"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			depl := args.Get(2).(*appsv1.Deployment)
			depl.Status.AvailableReplicas = 1
		}).
		Return(nil)

	ctx, cancel := context.WithTimeout(
		context.Background(), 2*time.Second)
	defer cancel()
	err := b.bootstrap(ctx, func(ctx context.Context) error {
		runManagerCalled = true
		runManagerCtx = ctx
		<-ctx.Done()
		return nil
	})
	require.NoError(t, err)
	assert.True(t, runManagerCalled)
	assert.Equal(t, context.Canceled, runManagerCtx.Err())
}
