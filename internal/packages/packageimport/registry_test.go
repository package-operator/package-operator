package packageimport

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"package-operator.run/package-operator/internal/packages/packagecontent"
)

func TestRegistry_DelayedPull(t *testing.T) {
	r := NewRegistry(map[string]string{
		"quay.io": "localhost:123",
	})
	ipm := &imagePullerMock{}
	r.pullImage = ipm.Pull

	f := packagecontent.Files{
		"test.yaml": []byte("test"),
	}
	ipm.
		On("Pull", mock.Anything, mock.Anything).
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
			ff, err := r.Pull(ctx, "quay.io/test123")
			require.NoError(t, err)
			assert.Equal(t, f, ff)
		}()
	}
	wg.Wait()

	ipm.AssertNumberOfCalls(t, "Pull", 1)
	ipm.AssertCalled(t, "Pull", mock.Anything, "localhost:123/test123:latest")
}

func TestRegistry_DelayedRequests(t *testing.T) {
	const (
		numRequests  = 3
		requestDelay = 5 * time.Millisecond
	)

	ipm := &imagePullerMock{}
	ipm.
		On("Pull", mock.Anything, mock.Anything).
		Return(packagecontent.Files{}, nil)

	r := NewRegistry(map[string]string{
		"quay.io": "localhost:123",
	})
	r.pullImage = ipm.Pull

	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Pull(ctx, "quay.io/test123")
		}()

		time.Sleep(requestDelay)
	}
	wg.Wait()

	ipm.AssertNumberOfCalls(t, "Pull", numRequests)
}

type imagePullerMock struct {
	mock.Mock
}

func (m *imagePullerMock) Pull(
	ctx context.Context, ref string,
) (packagecontent.Files, error) {
	args := m.Called(ctx, ref)
	return args.Get(0).(packagecontent.Files), args.Error(1)
}
