package packagetypes

import (
	"context"

	"github.com/google/go-containerregistry/pkg/crane"
)

// Insecure is an Option that allows image references to be fetched without TLS.
type WithInsecure struct{}

func (WithInsecure) ApplyToRegistry(opts *RegistryOptions) {
	opts.Insecure = true
}

type WithCraneOptions []crane.Option

func (copts WithCraneOptions) ApplyToRegistry(opts *RegistryOptions) {
	opts.CraneOptions = copts
}

// RegistryOptions configures registry behaviour.
type RegistryOptions struct {
	// Insecure is an Option that allows image references to be fetched and pushed without TLS.
	Insecure bool
	// Raw crane options.
	CraneOptions []crane.Option
}

// Returns RegistryOptions with defaults.
func DefaultRegistryOptions() RegistryOptions {
	return RegistryOptions{}
}

// Interface implemented by all registry options.
type RegistryOption interface {
	ApplyToRegistry(opts *RegistryOptions)
}

// Converts registry options to crane options.
func RegistryOptionsToCraneOptions(ctx context.Context, opts RegistryOptions) []crane.Option {
	internalOpts := []crane.Option{
		crane.WithContext(ctx),
	}
	if opts.Insecure {
		internalOpts = append(internalOpts, crane.Insecure)
	}
	internalOpts = append(internalOpts, opts.CraneOptions...)
	return internalOpts
}
