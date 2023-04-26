package testing

import (
	"github.com/stretchr/testify/mock"
)

type DigestResolverMock struct {
	mock.Mock
}

func (m *DigestResolverMock) ResolveDigest(ref string) (string, error) {
	args := m.Called(ref)

	return args.String(0), args.Error(1)
}
