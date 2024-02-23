package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	cacheDir = ".cache"

	defaultImageRegistry    = "quay.io/package-operator"
	imageRegistryEnvvarName = "IMAGE_REGISTRY"

	devClusterRegistryPort int32 = 5001
)

// Get image registry to use for tagging and pushing images
// from the envvar behind `imageRegistryEnvvarName`.
// The value is defaulted to the value of `defaultImageRegistry`.
func imageRegistry() string {
	imageRegistry, ok := os.LookupEnv(imageRegistryEnvvarName)
	if !ok {
		return defaultImageRegistry
	}
	return imageRegistry
}

// Extract hostname from image registry.
// For example: Using "quay.io/foobar" yields "quay.io".
func imageRegistryHost() string {
	u, err := url.Parse("http://" + imageRegistry())
	if err != nil {
		panic(fmt.Errorf("parsing hostname from image registry: %w", err))
	}
	return u.Host
}

// Extract namespace from image registry.
// For example: Using "quay.io/foobar" yields "foobar".
func imageRegistryNamespace() string {
	u, err := url.Parse("http://" + imageRegistry())
	if err != nil {
		panic(fmt.Errorf("parsing namespace from image registry: %w", err))
	}
	return strings.TrimPrefix(u.Path, "/")
}

// Build local registry url prefix with another hostPort.
// This is used for pushing images to the local dev registry in kind.
// For example (with `imageRegistry()` returning "quay.io/foobar"):
// Passing "localhost:3001" yields "localhost:3001/foobar".
func localRegistry(hostPort string) string {
	return hostPort + "/" + imageRegistryNamespace()
}
