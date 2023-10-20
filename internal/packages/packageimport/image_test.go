package packageimport

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages/packagecontent"
)

type pullerTestCase struct {
	name              string
	ref               string
	opts              []PullOption
	prepCranePullFn   func(t *testing.T, tc *pullerTestCase) cranePullFn
	prepImageFn       func(t *testing.T, tc *pullerTestCase) imageFn
	expectImageCalled bool

	// internal test state
	_cranePullCalled bool
	_imageCalled     bool
}

func TestPuller(t *testing.T) {
	t.Parallel()

	tcases := []pullerTestCase{
		{
			name:              "Plain",
			ref:               "example.com/image:latest",
			opts:              []PullOption{},
			expectImageCalled: true,
			prepCranePullFn: func(t *testing.T, tc *pullerTestCase) cranePullFn {
				t.Helper()
				return func(src string, opts ...crane.Option) (v1.Image, error) {
					tc._cranePullCalled = true
					assert.Equal(t, tc.ref, src)
					assert.Len(t, opts, 0)
					return nil, nil
				}
			},
			prepImageFn: mockImageExpectCalled,
		},
		{
			name: "WithAuthAssertCraneOpt",
			ref:  "example.com/private-image:latest",
			opts: []PullOption{
				WithPullSecret{
					Data: []byte(`{"auths":{"example.com":{"username":"example","password":"example"}}}`),
				},
			},
			expectImageCalled: true,
			prepCranePullFn: func(t *testing.T, tc *pullerTestCase) cranePullFn {
				t.Helper()
				return func(src string, opts ...crane.Option) (v1.Image, error) {
					tc._cranePullCalled = true
					assert.Equal(t, tc.ref, src)
					require.Len(t, opts, 1)
					return nil, nil
				}
			},
			prepImageFn: mockImageExpectCalled,
		},
	}

	for i := range tcases {
		tc := tcases[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := &Puller{
				cranePull: tc.prepCranePullFn(t, &tc),
				image:     tc.prepImageFn(t, &tc),
			}
			_, err := p.Pull(context.Background(), tc.ref, tc.opts...)
			require.NoError(t, err)
			require.True(t, tc._cranePullCalled, "crane pull called")
			require.Equal(t, tc.expectImageCalled, tc._imageCalled, "iamge called")
		})
	}
}

func mockImageExpectCalled(t *testing.T, tc *pullerTestCase) imageFn {
	t.Helper()
	return func(ctx context.Context, image v1.Image) (m packagecontent.Files, err error) {
		tc._imageCalled = true
		assert.Equal(t, nil, image)
		return nil, nil
	}
}
