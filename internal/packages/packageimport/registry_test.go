package packageimport

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/packages/packagecontent"
)

func TestRegistry_DelayedPull(t *testing.T) {
	t.Parallel()

	r := NewRegistry(map[string]string{
		"quay.io": "localhost:123",
	})
	ipm := &imagePullerMock{}
	r.pullImage = ipm.Pull

	f := packagecontent.Files{
		"test.yaml": []byte("test"),
	}
	ipm.
		On("Pull", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			time.Sleep(500 * time.Millisecond)
		}).
		Return(f, nil)

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ff, err := r.Pull(ctx, "quay.io/test123", corev1alpha1.PackageTypePackageOperator)
			require.NoError(t, err)
			assert.Equal(t, f, ff)
		}()
	}
	wg.Wait()

	ipm.AssertNumberOfCalls(t, "Pull", 1)
	ipm.AssertCalled(t, "Pull", mock.Anything, "localhost:123/test123:latest", corev1alpha1.PackageTypePackageOperator)
}

func TestRegistry_DelayedRequests(t *testing.T) {
	t.Parallel()

	const (
		numRequests  = 3
		requestDelay = 100 * time.Millisecond
	)

	ipm := &imagePullerMock{}
	ipm.
		On("Pull", mock.Anything, mock.Anything, mock.Anything).
		Return(packagecontent.Files{}, nil)

	r := NewRegistry(map[string]string{
		"quay.io": "localhost:123",
	})
	r.pullImage = ipm.Pull

	ctx := context.Background()
	var (
		wg sync.WaitGroup

		files     []packagecontent.Files
		filesLock sync.Mutex
	)
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, _ := r.Pull(ctx, "quay.io/test123", corev1alpha1.PackageTypePackageOperator)

			filesLock.Lock()
			defer filesLock.Unlock()
			files = append(files, f)
		}()

		time.Sleep(requestDelay)
	}
	wg.Wait()

	ipm.AssertNumberOfCalls(t, "Pull", numRequests)
	assert.Len(t, files, numRequests)

	// Ensure no two returned file maps are the same map object.
	maxRequestIndex := numRequests - 1
	for i := 0; i <= maxRequestIndex; i++ {
		for j := maxRequestIndex; j >= 0; j-- {
			af := files[i]
			bf := files[j]
			assert.NotSame(t, af, bf)
		}
	}
}

type imagePullerMock struct {
	mock.Mock
}

func (m *imagePullerMock) Pull(
	ctx context.Context, ref string,
	pkgType corev1alpha1.PackageType,
) (packagecontent.Files, error) {
	args := m.Called(ctx, ref, pkgType)
	return args.Get(0).(packagecontent.Files), args.Error(1)
}
