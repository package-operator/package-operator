package cmd

import (
	"github.com/google/go-containerregistry/pkg/crane"
)

type DigestResolver interface {
	ResolveDigest(ref string) (string, error)
}

type defaultDigestResolver struct{}

func (r *defaultDigestResolver) ResolveDigest(ref string) (string, error) {
	return crane.Digest(ref)
}
