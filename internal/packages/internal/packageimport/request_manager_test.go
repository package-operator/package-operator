package packageimport

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"package-operator.run/internal/packages/internal/packagetypes"
)

func TestRequestManager_DelayedPull(t *testing.T) {
	t.Parallel()

	r := NewRequestManager(map[string]string{
		"quay.io": "localhost:123",
	})
	ipm := &imagePullerMock{}
	r.pullImage = ipm.Pull

	pkg := &packagetypes.RawPackage{Files: packagetypes.Files{"test": []byte{}}}
	ipm.
		On("Pull", mock.Anything, mock.IsType("string")).
		Run(func(mock.Arguments) { time.Sleep(500 * time.Millisecond) }).
		Return(pkg, nil)

	ctx := context.Background()
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ff, err := r.Pull(ctx, "quay.io/test123")
			if err != nil {
				panic(err)
			}
			if !reflect.DeepEqual(pkg, ff) {
				panic("not equal")
			}
		}()
	}
	wg.Wait()

	ipm.AssertNumberOfCalls(t, "Pull", 1)
	ipm.AssertCalled(t, "Pull", mock.Anything, "localhost:123/test123:latest")
}

func TestRequestManager_DelayedRequests(t *testing.T) {
	t.Parallel()

	const (
		numRequests  = 3
		requestDelay = 100 * time.Millisecond
	)

	ipm := &imagePullerMock{}
	ipm.
		On("Pull", mock.Anything, mock.IsType("string")).
		Run(func(mock.Arguments) { time.Sleep(requestDelay) }).
		Return(&packagetypes.RawPackage{Files: packagetypes.Files{"test": nil}}, nil)

	r := NewRequestManager(map[string]string{
		"quay.io": "localhost:123",
	})
	r.pullImage = ipm.Pull

	ctx := context.Background()
	var (
		wg sync.WaitGroup

		pkg     []*packagetypes.RawPackage
		pkgLock sync.Mutex
	)
	for range numRequests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, err := r.Pull(ctx, "quay.io/test123")
			if err != nil {
				panic(err)
			}

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
	ctx context.Context, ref string, _ ...crane.Option,
) (*packagetypes.RawPackage, error) {
	args := m.Called(ctx, ref)
	return args.Get(0).(*packagetypes.RawPackage), args.Error(1)
}
