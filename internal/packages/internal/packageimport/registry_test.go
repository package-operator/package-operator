package packageimport

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestRegistry_DelayedPull(t *testing.T) {
	t.Parallel()

	r := NewRegistry(map[string]string{
		"quay.io": "localhost:123",
	})
	ipm := &imagePullerMock{}
	r.pullImage = ipm.Pull

	pkg := &packagetypes.RawPackage{Files: packagetypes.Files{"test": []byte{}}}
	ipm.
		On("Pull", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			time.Sleep(500 * time.Millisecond)
		}).
		Return(pkg, nil)

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ff, err := r.Pull(ctx, "quay.io/test123")
			require.NoError(t, err)
			assert.Equal(t, pkg, ff)
		}()
	}
	wg.Wait()

	ipm.AssertNumberOfCalls(t, "Pull", 1)
	ipm.AssertCalled(t, "Pull", mock.Anything, "localhost:123/test123:latest", mock.Anything)
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
		Run(func(args mock.Arguments) {
			time.Sleep(requestDelay)
		}).
		Return(&packagetypes.RawPackage{Files: packagetypes.Files{"test": nil}}, nil)

	r := NewRegistry(map[string]string{
		"quay.io": "localhost:123",
	})
	r.pullImage = ipm.Pull

	ctx := context.Background()
	var (
		wg sync.WaitGroup

		pkg     []*packagetypes.RawPackage
		pkgLock sync.Mutex
	)
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, err := r.Pull(ctx, "quay.io/test123")
			require.NoError(t, err)

			pkgLock.Lock()
			defer pkgLock.Unlock()
			pkg = append(pkg, f)
		}()
	}
	wg.Wait()

	ipm.AssertNumberOfCalls(t, "Pull", 1)
	assert.Len(t, pkg, numRequests)

	// Ensure no two returned file maps are the same map object.
	maxRequestIndex := numRequests - 1
	for i := 0; i <= maxRequestIndex; i++ {
		for j := maxRequestIndex; j >= 0; j-- {
			af := pkg[i].Files
			bf := pkg[j].Files
			assert.NotSame(t, af, bf)
		}
	}
}

type imagePullerMock struct {
	mock.Mock
}

func (m *imagePullerMock) Pull(
	ctx context.Context, ref string,
	opts ...crane.Option,
) (*packagetypes.RawPackage, error) {
	args := m.Called(ctx, ref, opts)
	return args.Get(0).(*packagetypes.RawPackage), args.Error(1)
}
