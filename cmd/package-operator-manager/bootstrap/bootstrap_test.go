package bootstrap

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/apis/manifests"
	"package-operator.run/internal/environment"
	"package-operator.run/internal/testutil"
)

const testBootstrapTimeout = 2 * time.Second

func TestBootstrapperBootstrap(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	var initCalled bool
	var fixCalled bool
	b := &Bootstrapper{
		Sink: environment.NewSink(c),

		log:    testr.New(t),
		client: c,
		init: func(context.Context) (bool, error) {
			initCalled = true
			return true, nil
		},
		fix: func(context.Context) error {
			fixCalled = true
			return nil
		},
	}
	b.SetEnvironment(&manifests.PackageEnvironment{
		Proxy: &manifests.PackageEnvironmentProxy{
			HTTPProxy:  "httpxxx",
			HTTPSProxy: "httpsxxx",
			NoProxy:    "noxxx",
		},
	})

	c.On("Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterPackage"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			cp := args.Get(2).(*corev1alpha1.ClusterPackage)
			cp.Generation = 5
			meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
				Type:               corev1alpha1.PackageAvailable,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: cp.Generation,
			})
		}).
		Return(nil)
	c.On("Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1.Deployment"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			depl := args.Get(2).(*appsv1.Deployment)
			depl.Status.AvailableReplicas = 1
			depl.Status.UpdatedReplicas = depl.Status.AvailableReplicas
		}).
		Return(nil)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, testBootstrapTimeout)
	defer cancel()
	err := b.Bootstrap(
		ctx, func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
	require.NoError(t, err)
	assert.True(t, initCalled)
	assert.True(t, fixCalled)

	assert.Equal(t, "httpxxx", os.Getenv("HTTP_PROXY"))
	assert.Equal(t, "httpsxxx", os.Getenv("HTTPS_PROXY"))
	assert.Equal(t, "noxxx", os.Getenv("NO_PROXY"))
}

func TestBootstrapper_bootstrap(t *testing.T) {
	t.Parallel()

	c := testutil.NewClient()
	// Create a test logger
	logger := testr.New(t)
	b := &Bootstrapper{
		client: c,
	}

	var (
		runManagerCalled bool
		runManagerCtx    context.Context
	)

	c.On("Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1alpha1.ClusterPackage"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			cp := args.Get(2).(*corev1alpha1.ClusterPackage)
			cp.Generation = 5
			meta.SetStatusCondition(&cp.Status.Conditions, metav1.Condition{
				Type:               corev1alpha1.PackageAvailable,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: cp.Generation,
			})
		}).
		Return(nil)
	c.On("Get", mock.Anything, mock.Anything,
		mock.AnythingOfType("*v1.Deployment"),
		mock.Anything).
		Run(func(args mock.Arguments) {
			depl := args.Get(2).(*appsv1.Deployment)
			depl.Status.AvailableReplicas = 1
			depl.Status.UpdatedReplicas = depl.Status.AvailableReplicas
		}).
		Return(nil)

	// Create a context with the logger
	ctx := logr.NewContext(context.Background(), logger)
	ctx, cancel := context.WithTimeout(ctx, testBootstrapTimeout)
	defer cancel()
	err := b.bootstrap(ctx, func(ctx context.Context) error {
		runManagerCalled = true
		runManagerCtx = ctx //nolint:fatcontext
		<-ctx.Done()
		t.Log("CONTEXT DONE")
		return nil
	})
	require.NoError(t, err)
	assert.True(t, runManagerCalled)
	assert.Equal(t, context.Canceled, runManagerCtx.Err())
}
