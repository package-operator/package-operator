package cmd

import (
	"github.com/google/go-containerregistry/pkg/crane"
)

type DigestResolver interface {
	ResolveDigest(ref string, opts ...ResolveDigestOption) (string, error)
}

type defaultDigestResolver struct{}

func (r *defaultDigestResolver) ResolveDigest(ref string, opts ...ResolveDigestOption) (string, error) {
	var cfg ResolveDigestConfig

	cfg.Option(opts...)

	var craneOpts []crane.Option

	if cfg.Insecure {
		craneOpts = append(craneOpts, crane.Insecure)
	}

	return crane.Digest(ref, craneOpts...)
}

type ResolveDigestConfig struct {
	Insecure bool
}

func (c *ResolveDigestConfig) Option(opts ...ResolveDigestOption) {
	for _, opt := range opts {
		opt.ConfigureResolveDigest(c)
	}
}

type ResolveDigestOption interface {
	ConfigureResolveDigest(*ResolveDigestConfig)
}
